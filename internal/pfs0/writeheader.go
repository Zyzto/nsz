package pfs0

import (
	"encoding/binary"
	"fmt"
	"strings"
)

func align20(n int64) int64 {
	return 0x20 - n%0x20
}

// FileRec is one member written after headerSize.
type FileRec struct {
	Name   string
	Offset int64 // absolute file offset
	Size   int64
}

// BuildHeaderExact builds PFS0 bytes with exact total header length headerSize (matches source layout when sizes align).
func BuildHeaderExact(headerSize int64, files []FileRec) ([]byte, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("pfs0: no files")
	}
	if files[0].Offset != headerSize {
		return nil, fmt.Errorf("pfs0: first file offset %d must equal header size %d", files[0].Offset, headerSize)
	}
	names := make([]string, len(files))
	for i := range files {
		names[i] = files[i].Name
	}
	strJoin := strings.Join(names, "\x00") + "\x00"
	meta := int64(0x10 + len(files)*0x18)
	strPad := headerSize - meta
	if strPad < int64(len(strJoin)) {
		return nil, fmt.Errorf("pfs0: headerSize %d too small for string table", headerSize)
	}
	strTable := strJoin + strings.Repeat("\x00", int(strPad-int64(len(strJoin))))

	h := make([]byte, 0, headerSize)
	h = append(h, []byte("PFS0")...)
	binary.LittleEndian.PutUint32(h[len(h):len(h)+4], uint32(len(files)))
	h = h[:len(h)+4]
	binary.LittleEndian.PutUint32(h[len(h):len(h)+4], uint32(strPad))
	h = h[:len(h)+4]
	h = append(h, 0, 0, 0, 0)

	strOff := 0
	for _, wf := range files {
		rel := wf.Offset - headerSize
		var b8 [8]byte
		binary.LittleEndian.PutUint64(b8[:], uint64(rel))
		h = append(h, b8[:]...)
		binary.LittleEndian.PutUint64(b8[:], uint64(wf.Size))
		h = append(h, b8[:]...)
		var b4 [4]byte
		binary.LittleEndian.PutUint32(b4[:], uint32(strOff))
		h = append(h, b4[:]...)
		h = append(h, 0, 0, 0, 0)
		strOff += len(wf.Name) + 1
	}
	h = append(h, []byte(strTable)...)
	if int64(len(h)) != headerSize {
		return nil, fmt.Errorf("pfs0: built header len %d want %d", len(h), headerSize)
	}
	return h, nil
}

// PaddedHeaderSize matches Python Pfs0.getPaddedHeaderSize for output file names (in order).
func PaddedHeaderSize(names []string) int64 {
	strJoin := strings.Join(names, "\x00") + "\x00"
	headerNP := int64(0x10 + len(names)*0x18 + len(strJoin))
	return headerNP + align20(headerNP)
}
