package hfs0

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// Entry is one member of an HFS0 partition (absolute offsets in the host file).
type Entry struct {
	Name   string
	Offset int64
	Size   int64
}

const (
	entrySize = 0x40
	maxFiles  = 100000
	maxStrTab = 64 << 20
	DataStart = 0x8000 // upstream Hfs0Stream reserves this much before payload
)

// ParseFrom parses HFS0 from a ReaderAt view of one partition (offsets 0..maxLen).
// Entry.Offset is absolute within that partition (byte offset from partition start).
func ParseFrom(r io.ReaderAt, maxLen int64) ([]Entry, int64, error) {
	if maxLen < 0x10 {
		return nil, 0, fmt.Errorf("hfs0: partition too small")
	}
	readAt := func(off int64, p []byte) error {
		n, err := r.ReadAt(p, off)
		if err != nil {
			return err
		}
		if n != len(p) {
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	hdr := make([]byte, 0x10)
	if err := readAt(0, hdr); err != nil {
		return nil, 0, err
	}
	if string(hdr[0:4]) != "HFS0" {
		return nil, 0, fmt.Errorf("hfs0: invalid magic %q", hdr[0:4])
	}
	fileCount := binary.LittleEndian.Uint32(hdr[4:8])
	strTabSize := binary.LittleEndian.Uint32(hdr[8:12])
	if fileCount == 0 || fileCount > maxFiles {
		return nil, 0, fmt.Errorf("hfs0: bad file count %d", fileCount)
	}
	if strTabSize > maxStrTab {
		return nil, 0, fmt.Errorf("hfs0: string table too large")
	}
	headerSize := int64(0x10 + uint32(fileCount)*entrySize + strTabSize)
	if headerSize > maxLen {
		return nil, 0, fmt.Errorf("hfs0: header exceeds partition")
	}
	strTable := make([]byte, strTabSize)
	if err := readAt(int64(0x10+uint32(fileCount)*entrySize), strTable); err != nil {
		return nil, 0, err
	}

	type raw struct {
		relOff, sz int64
		nameOff    uint32
	}
	raws := make([]raw, fileCount)
	row := make([]byte, entrySize)
	for i := int(fileCount) - 1; i >= 0; i-- {
		rowOff := int64(0x10 + i*entrySize)
		if err := readAt(rowOff, row); err != nil {
			return nil, 0, err
		}
		rel := int64(binary.LittleEndian.Uint64(row[0:8]))
		sz := int64(binary.LittleEndian.Uint64(row[8:16]))
		nameOff := binary.LittleEndian.Uint32(row[16:20])
		raws[i] = raw{relOff: rel, sz: sz, nameOff: nameOff}
	}

	names := make([]string, fileCount)
	stringEnd := int(strTabSize)
	for i := int(fileCount) - 1; i >= 0; i-- {
		rw := raws[i]
		no := int(rw.nameOff)
		if no < 0 || no >= stringEnd || no >= len(strTable) {
			return nil, 0, fmt.Errorf("hfs0: bad name offset")
		}
		names[i] = readCString(strTable[no:stringEnd])
		stringEnd = no
	}

	entries := make([]Entry, 0, fileCount)
	for i := range raws {
		rw := raws[i]
		abs := headerSize + rw.relOff
		if rw.sz < 0 || abs < 0 || abs+rw.sz > maxLen {
			return nil, 0, fmt.Errorf("hfs0: entry %q out of range", names[i])
		}
		entries = append(entries, Entry{Name: names[i], Offset: abs, Size: rw.sz})
	}
	return entries, headerSize, nil
}

// ParseAt reads HFS0 at absolute offset base in f (entire partition; maxLen caps read).
// Entry.Offset is absolute in f.
func ParseAt(f *os.File, base int64, maxLen int64) ([]Entry, int64, error) {
	entries, hs, err := ParseFrom(io.NewSectionReader(f, base, maxLen), maxLen)
	if err != nil {
		return nil, 0, err
	}
	for i := range entries {
		entries[i].Offset += base
	}
	return entries, hs, err
}

func readCString(b []byte) string {
	i := 0
	for i < len(b) && b[i] != 0 {
		i++
	}
	return strings.TrimSpace(string(b[:i]))
}

// Align200 matches Python allign0x200 + size (padding to 0x200 after payload).
func Align200(n int64) int64 {
	const m = 0x200
	r := n % m
	if r == 0 {
		return n + m
	}
	return n + (m - r)
}
