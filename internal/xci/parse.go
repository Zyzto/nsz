package xci

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Header holds fields needed to locate the root HFS0 (Python Xci.readHeader / open).
type Header struct {
	First0x200   []byte // bytes [0,0x200) copied into XCZ like upstream XciStream
	HeaderOffset int64  // 0 or 0x1000
	HFS0Offset   int64  // absolute file offset of root HFS0
}

const (
	fullXCIHeaderOffset = 0x1000
)

// ParseOpened reads XCI layout from an open file (position can be anywhere).
func ParseOpened(f *os.File) (*Header, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	sz := fi.Size()
	if sz < 0x200 {
		return nil, fmt.Errorf("xci: file too small")
	}
	first := make([]byte, 0x200)
	if _, err := f.ReadAt(first, 0); err != nil {
		return nil, err
	}
	full, err := isFullXCI(f)
	if err != nil {
		return nil, err
	}
	var hdrOff int64
	if full {
		hdrOff = fullXCIHeaderOffset
	} else {
		hdrOff = 0
	}
	if sz < hdrOff+0x140 {
		return nil, fmt.Errorf("xci: truncated header")
	}
	var hfsRel int64
	if err := binary.Read(io.NewSectionReader(f, hdrOff+0x130, 8), binary.LittleEndian, &hfsRel); err != nil {
		return nil, err
	}
	abs := hdrOff + hfsRel
	if abs < 0 || abs >= sz {
		return nil, fmt.Errorf("xci: invalid hfs0 offset %d (size %d)", abs, sz)
	}
	return &Header{First0x200: first, HeaderOffset: hdrOff, HFS0Offset: abs}, nil
}

func isFullXCI(f *os.File) (bool, error) {
	var m [4]byte
	if _, err := f.ReadAt(m[:], 0x100); err != nil {
		return false, err
	}
	return string(m[:]) != "HEAD", nil
}
