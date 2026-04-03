package ncz

import (
	"encoding/binary"
	"fmt"
	"io"
)

// InspectResult summarizes an NCZ payload without decompressing.
type InspectResult struct {
	DecompressedSize int64
	BlockCompressed  bool
	Sections         []Section
}

// Inspect reads the NCZ section table and detects solid vs block payload (Python NCZ header path).
func Inspect(rs io.ReadSeeker) (*InspectResult, error) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if _, err := rs.Seek(UncompressibleHeaderSize, io.SeekStart); err != nil {
		return nil, err
	}
	magic := make([]byte, 8)
	if _, err := io.ReadFull(rs, magic); err != nil {
		return nil, err
	}
	if string(magic) != "NCZSECTN" {
		return nil, fmt.Errorf("ncz: missing NCZSECTN magic")
	}
	var sectionCount int64
	if err := binary.Read(rs, binary.LittleEndian, &sectionCount); err != nil {
		return nil, err
	}
	if sectionCount < 0 || sectionCount > 4096 {
		return nil, fmt.Errorf("ncz: invalid section count %d", sectionCount)
	}
	sections := make([]Section, sectionCount)
	for i := range sections {
		var e error
		sections[i], e = ReadSection(rs)
		if e != nil {
			return nil, e
		}
	}
	if len(sections) > 0 && sections[0].Offset > UncompressibleHeaderSize {
		fs := Section{
			Offset:     UncompressibleHeaderSize,
			Size:       sections[0].Offset - UncompressibleHeaderSize,
			CryptoType: 1,
		}
		sections = append([]Section{fs}, sections...)
	}
	var ncaSize int64 = UncompressibleHeaderSize
	for _, s := range sections {
		if s.Offset >= UncompressibleHeaderSize {
			ncaSize += s.Size
		}
	}

	pos, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	peek := make([]byte, 8)
	if _, err := io.ReadFull(rs, peek); err != nil {
		return nil, err
	}
	if _, err := rs.Seek(pos, io.SeekStart); err != nil {
		return nil, err
	}
	block := string(peek) == "NCZBLOCK"

	return &InspectResult{
		DecompressedSize: ncaSize,
		BlockCompressed:  block,
		Sections:         sections,
	}, nil
}
