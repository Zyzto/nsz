package ncz

import (
	"encoding/binary"
	"fmt"
	"io"
)

const UncompressibleHeaderSize = 0x4000

type Section struct {
	Offset     int64
	Size       int64
	CryptoType int64
	CryptoKey  [16]byte
	CryptoCtr  [16]byte
}

type FakeSection struct {
	Offset int64
	Size   int64
}

type BlockHeader struct {
	Magic                [8]byte
	Version              int8
	Type                 int8
	Unused               int8
	BlockSizeExponent    int8
	NumberOfBlocks       int32
	DecompressedSize     int64
	CompressedBlockSizes []int32
}

func ReadSection(r io.Reader) (Section, error) {
	var s Section
	if err := binary.Read(r, binary.LittleEndian, &s.Offset); err != nil {
		return s, err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.Size); err != nil {
		return s, err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.CryptoType); err != nil {
		return s, err
	}
	var pad int64
	if err := binary.Read(r, binary.LittleEndian, &pad); err != nil {
		return s, err
	}
	if _, err := io.ReadFull(r, s.CryptoKey[:]); err != nil {
		return s, err
	}
	if _, err := io.ReadFull(r, s.CryptoCtr[:]); err != nil {
		return s, err
	}
	return s, nil
}

func ReadBlockHeader(r io.Reader) (*BlockHeader, error) {
	var b BlockHeader
	if _, err := io.ReadFull(r, b.Magic[:]); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Version); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Type); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Unused); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.BlockSizeExponent); err != nil {
		return nil, err
	}
	if b.BlockSizeExponent < 14 || b.BlockSizeExponent > 32 {
		return nil, fmt.Errorf("ncz: block size exponent %d out of range", b.BlockSizeExponent)
	}
	if err := binary.Read(r, binary.LittleEndian, &b.NumberOfBlocks); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.DecompressedSize); err != nil {
		return nil, err
	}
	b.CompressedBlockSizes = make([]int32, b.NumberOfBlocks)
	for i := range b.CompressedBlockSizes {
		if err := binary.Read(r, binary.LittleEndian, &b.CompressedBlockSizes[i]); err != nil {
			return nil, err
		}
	}
	return &b, nil
}
