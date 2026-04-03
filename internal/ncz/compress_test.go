package ncz

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestSolidCompressDecompressRoundTrip(t *testing.T) {
	body := bytes.Repeat([]byte{0xAB}, 50_000)
	nca := make([]byte, UncompressibleHeaderSize+len(body))
	copy(nca[UncompressibleHeaderSize:], body)
	for i := range nca[:UncompressibleHeaderSize] {
		nca[i] = byte(i & 0xff) // arbitrary NCA header region (preserved verbatim)
	}

	var buf bytes.Buffer
	if err := WriteSolidNCZ(&buf, nca, SolidEncodeOptions{Level: 3, Threads: 1}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	r := bytes.NewReader(buf.Bytes())
	_, _, err := Decompress(r, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), nca) {
		t.Fatalf("round-trip mismatch at %d", firstDiff(out.Bytes(), nca))
	}
}

func TestSolidCompressMatchesBuildSolidNCZPayload(t *testing.T) {
	body := bytes.Repeat([]byte{0xCD}, 12345)
	nca := make([]byte, UncompressibleHeaderSize+len(body))
	copy(nca[UncompressibleHeaderSize:], body)
	// buildSolidNCZ uses a zeroed 0x4000 header; keep header zero so decompressed NCA matches.

	var wbuf bytes.Buffer
	if err := WriteSolidNCZ(&wbuf, nca, SolidEncodeOptions{Level: 3, Threads: 1}); err != nil {
		t.Fatal(err)
	}
	ref, err := buildSolidNCZ(body)
	if err != nil {
		t.Fatal(err)
	}
	// Same logical NCZ; zstd bitstream may differ by encoder.
	var decW, decR bytes.Buffer
	if _, _, err := Decompress(bytes.NewReader(wbuf.Bytes()), &decW, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Decompress(bytes.NewReader(ref), &decR, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decW.Bytes(), decR.Bytes()) {
		t.Fatal("decompressed NCA differs between WriteSolidNCZ and buildSolidNCZ")
	}
}

func TestBlockReaderUncompressedLastBlock(t *testing.T) {
	blockExp := int8(8) // 256 bytes per block
	blockSize := int64(1 << blockExp)
	p1 := bytes.Repeat([]byte{1}, int(blockSize))
	p2 := bytes.Repeat([]byte{2}, int(blockSize/2)) // 128 bytes last block
	var comp1 bytes.Buffer
	zw, err := zstd.NewWriter(&comp1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(p1); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	// Last block stored uncompressed (Python: len(compressed) >= decompressedBlockSize)
	sizes := []int32{int32(comp1.Len()), int32(len(p2))}
	var payload bytes.Buffer
	if _, err := payload.Write(comp1.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Write(p2); err != nil {
		t.Fatal(err)
	}
	bh := &BlockHeader{
		Magic:                [8]byte{'N', 'C', 'Z', 'B', 'L', 'O', 'C', 'K'},
		BlockSizeExponent:    blockExp,
		NumberOfBlocks:       2,
		DecompressedSize:     blockSize + int64(len(p2)),
		CompressedBlockSizes: sizes,
	}
	full := bytes.NewReader(payload.Bytes())
	br, err := NewBlockReader(full, bh, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	buf := make([]byte, blockSize+int64(len(p2)))
	n, err := io.ReadFull(br, buf)
	if err != nil {
		t.Fatal(err)
	}
	wantN := blockSize + int64(len(p2))
	if int64(n) != wantN {
		t.Fatalf("read %d want %d", n, wantN)
	}
	if !bytes.Equal(buf[:blockSize], p1) {
		t.Fatal("block1 mismatch")
	}
	if !bytes.Equal(buf[blockSize:], p2) {
		t.Fatal("block2 mismatch")
	}
}

// TestBlockReaderPaddedUncompressedSlot verifies parity with Python BlockDecompressorReader:
// when CompressedBlockSizeList[i] >= decompressedBlockSize, only the first
// decompressedBlockSize bytes are payload; extra on-disk bytes are padding.
func TestBlockReaderPaddedUncompressedSlot(t *testing.T) {
	blockExp := int8(8)
	logical := bytes.Repeat([]byte{0xAA}, 64)
	padding := bytes.Repeat([]byte{0xEE}, 24)
	onDisk := append(append([]byte(nil), logical...), padding...)
	sizes := []int32{int32(len(onDisk))}
	bh := &BlockHeader{
		Magic:                [8]byte{'N', 'C', 'Z', 'B', 'L', 'O', 'C', 'K'},
		BlockSizeExponent:    blockExp,
		NumberOfBlocks:       1,
		DecompressedSize:     64, // one short final "block" in upstream terms
		CompressedBlockSizes: sizes,
	}
	br, err := NewBlockReader(bytes.NewReader(onDisk), bh, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	out := make([]byte, 64)
	if _, err := io.ReadFull(br, out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, logical) {
		t.Fatalf("want logical payload only, got %x", out)
	}
	if _, err := br.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("after logical end want EOF, got %v", err)
	}
}

func TestBlockReaderLastBlockPaddedAfterCompressed(t *testing.T) {
	blockExp := int8(8)
	blockSize := int64(1 << blockExp)
	p1 := bytes.Repeat([]byte{1}, int(blockSize))
	logical2 := bytes.Repeat([]byte{2}, 100)
	pad2 := bytes.Repeat([]byte{0xFF}, 40)
	disk2 := append(append([]byte(nil), logical2...), pad2...)

	var comp1 bytes.Buffer
	zw, err := zstd.NewWriter(&comp1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(p1); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var payload bytes.Buffer
	if _, err := payload.Write(comp1.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Write(disk2); err != nil {
		t.Fatal(err)
	}
	bh := &BlockHeader{
		Magic:                [8]byte{'N', 'C', 'Z', 'B', 'L', 'O', 'C', 'K'},
		BlockSizeExponent:    blockExp,
		NumberOfBlocks:       2,
		DecompressedSize:     blockSize + 100,
		CompressedBlockSizes: []int32{int32(comp1.Len()), int32(len(disk2))},
	}
	br, err := NewBlockReader(bytes.NewReader(payload.Bytes()), bh, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	buf := make([]byte, blockSize+100)
	if _, err := io.ReadFull(br, buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:blockSize], p1) {
		t.Fatal("block1 mismatch")
	}
	if !bytes.Equal(buf[blockSize:], logical2) {
		t.Fatal("block2 logical mismatch")
	}
}
