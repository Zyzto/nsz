package ncz

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

// BlockReader implements random access over block-compressed NCZ payload (upstream NSZ behavior).
type BlockReader struct {
	r                 io.ReadSeeker
	hdr               *BlockHeader
	blockSize         int64
	compressedOffsets []int64
	position          int64
	curBlockID        int
	curBlock          []byte
	dec               *zstd.Decoder
}

func NewBlockReader(r io.ReadSeeker, hdr *BlockHeader, dataStart int64) (*BlockReader, error) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	br := &BlockReader{
		r:         r,
		hdr:       hdr,
		blockSize: int64(1) << uint(hdr.BlockSizeExponent),
		dec:       dec,
	}
	br.compressedOffsets = make([]int64, len(hdr.CompressedBlockSizes))
	off := dataStart
	for i, sz := range hdr.CompressedBlockSizes {
		br.compressedOffsets[i] = off
		off += int64(sz)
	}
	return br, nil
}

func (b *BlockReader) Close() {
	if b.dec != nil {
		b.dec.Close()
		b.dec = nil
	}
}

func (b *BlockReader) decompressBlock(blockID int) ([]byte, error) {
	if b.curBlockID == blockID && b.curBlock != nil {
		return b.curBlock, nil
	}
	decompressedBlockSize := b.blockSize
	if blockID >= len(b.compressedOffsets)-1 {
		if blockID >= len(b.compressedOffsets) {
			return nil, io.EOF
		}
		rem := b.hdr.DecompressedSize % b.blockSize
		if rem > 0 {
			decompressedBlockSize = rem
		}
	}
	if _, err := b.r.Seek(b.compressedOffsets[blockID], io.SeekStart); err != nil {
		return nil, err
	}
	compSz := int64(b.hdr.CompressedBlockSizes[blockID])
	buf := make([]byte, compSz)
	if _, err := io.ReadFull(b.r, buf); err != nil {
		return nil, err
	}
	var out []byte
	if compSz < decompressedBlockSize {
		var err error
		out, err = b.dec.DecodeAll(buf, nil)
		if err != nil {
			return nil, err
		}
		b.curBlock = out
	} else {
		b.curBlock = append([]byte(nil), buf...)
	}
	b.curBlockID = blockID
	return b.curBlock, nil
}

// Read at current Position (decompressed space).
func (b *BlockReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	nTotal := 0
	for nTotal < len(p) {
		blockOffset := b.position % b.blockSize
		blockID := int(b.position / b.blockSize)
		if blockID >= len(b.compressedOffsets) {
			break
		}
		blk, err := b.decompressBlock(blockID)
		if err != nil {
			return nTotal, err
		}
		if int(blockOffset) >= len(blk) {
			break
		}
		chunk := blk[blockOffset:]
		need := len(p) - nTotal
		if len(chunk) > need {
			chunk = chunk[:need]
		}
		copy(p[nTotal:], chunk)
		nTotal += len(chunk)
		b.position += int64(len(chunk))
	}
	if nTotal == 0 {
		return 0, io.EOF
	}
	return nTotal, nil
}

func (b *BlockReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		b.position = offset
	case io.SeekCurrent:
		b.position += offset
	case io.SeekEnd:
		b.position = b.hdr.DecompressedSize + offset
	default:
		return 0, io.EOF
	}
	return b.position, nil
}
