package hfs0

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// FileRec is one member to place in a new HFS0 image (payload only).
type FileRec struct {
	Name string
	Data []byte
}

// TocFile is one member for TOC generation (streaming builds).
type TocFile struct {
	Name   string
	RelOff int64
	Size   int64
}

// HeaderLen returns the HFS0 header size for the given member names (in order).
func HeaderLen(names []string) int64 {
	strTable := strings.Join(names, "\x00") + "\x00"
	return int64(0x10 + len(names)*entrySize + len(strTable))
}

// BuildHeaderBytes builds the HFS0 TOC + string table (payload not included).
func BuildHeaderBytes(parts []TocFile) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("hfs0: empty file list")
	}
	names := make([]string, len(parts))
	for i := range parts {
		names[i] = parts[i].Name
	}
	strTable := strings.Join(names, "\x00") + "\x00"
	headerSize := int64(0x10 + len(parts)*entrySize + len(strTable))
	if headerSize > DataStart {
		return nil, fmt.Errorf("hfs0: TOC too large (%d) for fixed data start 0x%x", headerSize, DataStart)
	}
	return buildHeaderTOC(parts, strTable, headerSize)
}

// Build constructs a full HFS0 partition (header at 0, payload from DataStart).
func Build(files []FileRec) ([]byte, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("hfs0: empty file list")
	}
	strTable := strings.Join(fileNames(files), "\x00") + "\x00"
	headerSize := int64(0x10 + len(files)*entrySize + len(strTable))
	if headerSize > DataStart {
		return nil, fmt.Errorf("hfs0: TOC too large (%d) for fixed data start 0x%x", headerSize, DataStart)
	}
	var pay int64
	for i := range files {
		pay += int64(len(files[i].Data))
	}
	total := DataStart + pay
	out := make([]byte, total)
	off := int64(DataStart)
	parts := make([]TocFile, len(files))
	for i := range files {
		parts[i] = TocFile{Name: files[i].Name, RelOff: off - headerSize, Size: int64(len(files[i].Data))}
		n := copy(out[off:], files[i].Data)
		if n != len(files[i].Data) {
			return nil, fmt.Errorf("hfs0: copy")
		}
		off += int64(len(files[i].Data))
	}
	hdr, err := buildHeaderTOC(parts, strTable, headerSize)
	if err != nil {
		return nil, err
	}
	copy(out, hdr)
	return out, nil
}

func fileNames(files []FileRec) []string {
	s := make([]string, len(files))
	for i := range files {
		s[i] = files[i].Name
	}
	return s
}

func buildHeaderTOC(parts []TocFile, strTable string, headerSize int64) ([]byte, error) {
	h := make([]byte, 0, headerSize)
	h = append(h, []byte("HFS0")...)
	var u32 [4]byte
	binary.LittleEndian.PutUint32(u32[:], uint32(len(parts)))
	h = append(h, u32[:]...)
	binary.LittleEndian.PutUint32(u32[:], uint32(len(strTable)))
	h = append(h, u32[:]...)
	h = append(h, 0, 0, 0, 0)
	strOff := 0
	for _, wf := range parts {
		var b8 [8]byte
		binary.LittleEndian.PutUint64(b8[:], uint64(wf.RelOff))
		h = append(h, b8[:]...)
		binary.LittleEndian.PutUint64(b8[:], uint64(wf.Size))
		h = append(h, b8[:]...)
		var b4 [4]byte
		binary.LittleEndian.PutUint32(b4[:], uint32(strOff))
		h = append(h, b4[:]...)
		h = append(h, 0, 0, 0, 0) // hashed region size = 0
		h = append(h, make([]byte, 8)...)
		h = append(h, make([]byte, 0x20)...) // sha256 placeholder
		strOff += len(wf.Name) + 1
	}
	h = append(h, []byte(strTable)...)
	if int64(len(h)) != headerSize {
		return nil, fmt.Errorf("hfs0: header size %d want %d", len(h), headerSize)
	}
	return h, nil
}
