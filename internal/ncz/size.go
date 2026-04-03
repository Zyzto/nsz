package ncz

import (
	"encoding/binary"
	"fmt"
	"io"
)

// DecompressedSize returns total NCA size after decompress (Python __getDecompressedNczSize).
func DecompressedSize(rs io.ReadSeeker) (int64, error) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	if _, err := rs.Seek(UncompressibleHeaderSize, io.SeekStart); err != nil {
		return 0, err
	}
	magic := make([]byte, 8)
	if _, err := io.ReadFull(rs, magic); err != nil {
		return 0, err
	}
	if string(magic) != "NCZSECTN" {
		return 0, fmt.Errorf("ncz: missing NCZSECTN")
	}
	var sectionCount int64
	if err := binary.Read(rs, binary.LittleEndian, &sectionCount); err != nil {
		return 0, err
	}
	if sectionCount < 0 || sectionCount > 4096 {
		return 0, fmt.Errorf("ncz: invalid section count %d", sectionCount)
	}
	sections := make([]Section, sectionCount)
	for i := range sections {
		var e error
		sections[i], e = ReadSection(rs)
		if e != nil {
			return 0, e
		}
	}
	if len(sections) > 0 && sections[0].Offset > UncompressibleHeaderSize {
		fs := Section{Offset: UncompressibleHeaderSize, Size: sections[0].Offset - UncompressibleHeaderSize, CryptoType: 1}
		sections = append([]Section{fs}, sections...)
	}
	var ncaSize int64 = UncompressibleHeaderSize
	for _, s := range sections {
		if s.Offset >= UncompressibleHeaderSize {
			ncaSize += s.Size
		}
	}
	return ncaSize, nil
}
