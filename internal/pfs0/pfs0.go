package pfs0

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// Entry is one file inside a PFS0 container.
type Entry struct {
	Name   string
	Offset int64 // absolute offset in container file
	Size   int64
}

// Read holds a parsed PFS0 container.
type Read struct {
	Path         string
	HeaderSize   int64
	StrTableSize uint32
	Entries      []Entry
	f            *os.File
}

// OpenPFS0 parses a PFS0 container at path (e.g. .nsp / .nsz).
func OpenPFS0(path string) (*Read, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	pr := &Read{Path: path, f: f}
	if err := pr.parse(); err != nil {
		f.Close()
		return nil, err
	}
	return pr, nil
}

func (p *Read) Close() error {
	if p.f == nil {
		return nil
	}
	err := p.f.Close()
	p.f = nil
	return err
}

func (p *Read) parse() error {
	var magic [4]byte
	if _, err := io.ReadFull(p.f, magic[:]); err != nil {
		return err
	}
	if string(magic[:]) != "PFS0" {
		return fmt.Errorf("pfs0: invalid magic %q", magic)
	}
	var fileCount, strTabSize uint32
	if err := binary.Read(p.f, binary.LittleEndian, &fileCount); err != nil {
		return err
	}
	if err := binary.Read(p.f, binary.LittleEndian, &strTabSize); err != nil {
		return err
	}
	const maxFiles = 100000
	if fileCount == 0 || fileCount > maxFiles {
		return fmt.Errorf("pfs0: invalid file count %d", fileCount)
	}
	const maxStrTable = 64 << 20 // 64 MiB
	if strTabSize > maxStrTable {
		return fmt.Errorf("pfs0: string table too large (%d)", strTabSize)
	}
	p.StrTableSize = strTabSize
	var _pad uint32
	if err := binary.Read(p.f, binary.LittleEndian, &_pad); err != nil {
		return err
	}
	if _, err := p.f.Seek(int64(0x10+fileCount*0x18), io.SeekStart); err != nil {
		return err
	}
	strTable := make([]byte, strTabSize)
	if _, err := io.ReadFull(p.f, strTable); err != nil {
		return err
	}
	headerSize := int64(0x10 + uint32(fileCount)*0x18 + strTabSize)

	type raw struct {
		off, sz int64
		nameOff uint32
	}
	raws := make([]raw, fileCount)
	for i := int(fileCount) - 1; i >= 0; i-- {
		if _, err := p.f.Seek(int64(0x10+i*0x18), io.SeekStart); err != nil {
			return err
		}
		var off, sz int64
		var nameOff, junk uint32
		if err := binary.Read(p.f, binary.LittleEndian, &off); err != nil {
			return err
		}
		if err := binary.Read(p.f, binary.LittleEndian, &sz); err != nil {
			return err
		}
		if err := binary.Read(p.f, binary.LittleEndian, &nameOff); err != nil {
			return err
		}
		if err := binary.Read(p.f, binary.LittleEndian, &junk); err != nil {
			return err
		}
		raws[i] = raw{off: off, sz: sz, nameOff: nameOff}
	}
	p.HeaderSize = headerSize
	// Names must be sliced like Python Pfs0.open: walk TOC from last row to first so
	// each name is stringTable[nameOffset:stringEndExclusive].
	names := make([]string, fileCount)
	stringEnd := int(strTabSize)
	if stringEnd > len(strTable) {
		return fmt.Errorf("pfs0: string table size %d exceeds read %d", stringEnd, len(strTable))
	}
	for i := int(fileCount) - 1; i >= 0; i-- {
		r := raws[i]
		no := int(r.nameOff)
		if no < 0 || no >= stringEnd || no >= len(strTable) {
			return fmt.Errorf("pfs0: invalid name offset %d (end %d)", r.nameOff, stringEnd)
		}
		names[i] = readCString(strTable[no:stringEnd])
		stringEnd = no
	}
	p.Entries = make([]Entry, 0, fileCount)
	for i := range raws {
		r := raws[i]
		p.Entries = append(p.Entries, Entry{
			Name:   names[i],
			Offset: headerSize + r.off,
			Size:   r.sz,
		})
	}
	return nil
}

func readCString(b []byte) string {
	i := 0
	for i < len(b) && b[i] != 0 {
		i++
	}
	return strings.TrimSpace(string(b[:i]))
}

// OpenSection returns an io.ReadSeeker for entry payload.
func (p *Read) OpenSection(e Entry) (io.ReadSeeker, error) {
	return &sectionReader{f: p.f, start: e.Offset, size: e.Size}, nil
}

type sectionReader struct {
	f     *os.File
	start int64
	size  int64
	pos   int64
}

func (s *sectionReader) Seek(offset int64, whence int) (int64, error) {
	var np int64
	switch whence {
	case io.SeekStart:
		np = offset
	case io.SeekCurrent:
		np = s.pos + offset
	case io.SeekEnd:
		np = s.size + offset
	default:
		return 0, fmt.Errorf("pfs0: bad whence")
	}
	if np < 0 || np > s.size {
		return 0, fmt.Errorf("pfs0: seek out of range")
	}
	s.pos = np
	return np, nil
}

func (s *sectionReader) Read(b []byte) (int, error) {
	if s.pos >= s.size {
		return 0, io.EOF
	}
	n := len(b)
	rem := s.size - s.pos
	if int64(n) > rem {
		n = int(rem)
	}
	if n == 0 {
		return 0, io.EOF
	}
	m, err := s.f.ReadAt(b[:n], s.start+s.pos)
	s.pos += int64(m)
	return m, err
}
