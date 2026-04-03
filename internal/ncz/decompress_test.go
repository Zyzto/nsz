package ncz

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// buildSolidNCZ creates a minimal solid NCZ: one section at 0x4000, cryptoType 1 (plaintext body).
func buildSolidNCZ(payload []byte) ([]byte, error) {
	var b bytes.Buffer
	hdr := make([]byte, UncompressibleHeaderSize)
	if _, err := b.Write(hdr); err != nil {
		return nil, err
	}
	if _, err := b.Write([]byte("NCZSECTN")); err != nil {
		return nil, err
	}
	secCount := int64(1)
	if err := binary.Write(&b, binary.LittleEndian, secCount); err != nil {
		return nil, err
	}
	off := int64(UncompressibleHeaderSize)
	sz := int64(len(payload))
	if err := binary.Write(&b, binary.LittleEndian, off); err != nil {
		return nil, err
	}
	if err := binary.Write(&b, binary.LittleEndian, sz); err != nil {
		return nil, err
	}
	ct := int64(1)
	if err := binary.Write(&b, binary.LittleEndian, ct); err != nil {
		return nil, err
	}
	var pad int64
	if err := binary.Write(&b, binary.LittleEndian, pad); err != nil {
		return nil, err
	}
	if _, err := b.Write(make([]byte, 32)); err != nil {
		return nil, err
	}
	zw, err := zstd.NewWriter(&b)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(payload); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func TestDecompressSolidRoundTrip(t *testing.T) {
	want := bytes.Repeat([]byte{0xCD}, 12345)
	raw, err := buildSolidNCZ(want)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	r := bytes.NewReader(raw)
	_, hex, err := Decompress(r, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := out.Bytes()
	if len(got) < UncompressibleHeaderSize+len(want) {
		t.Fatalf("short output: %d", len(got))
	}
	body := got[UncompressibleHeaderSize:]
	if !bytes.Equal(body, want) {
		t.Fatalf("payload mismatch, first diff at %d", firstDiff(body, want))
	}
	if len(hex) != 64 {
		t.Fatalf("hash length %d", len(hex))
	}
}

func firstDiff(a, b []byte) int {
	for i := range a {
		if i >= len(b) || a[i] != b[i] {
			return i
		}
	}
	return len(a)
}

func TestBlockReaderSequentialRead(t *testing.T) {
	// Two blocks: compress small payloads
	blockExp := int8(16)
	blockSize := int64(1 << blockExp)
	p1 := bytes.Repeat([]byte{1}, int(blockSize))
	p2 := bytes.Repeat([]byte{2}, int(blockSize/2))
	var comp1, comp2 bytes.Buffer
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
	zw2, err := zstd.NewWriter(&comp2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw2.Write(p2); err != nil {
		t.Fatal(err)
	}
	if err := zw2.Close(); err != nil {
		t.Fatal(err)
	}
	sizes := []int32{int32(comp1.Len()), int32(comp2.Len())}
	var payload bytes.Buffer
	if _, err := payload.Write(comp1.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := payload.Write(comp2.Bytes()); err != nil {
		t.Fatal(err)
	}

	bh := &BlockHeader{
		Magic:                [8]byte{'N', 'C', 'Z', 'B', 'L', 'O', 'C', 'K'},
		BlockSizeExponent:    blockExp,
		NumberOfBlocks:       2,
		DecompressedSize:     blockSize + blockSize/2,
		CompressedBlockSizes: sizes,
	}
	full := io.NewSectionReader(bytes.NewReader(payload.Bytes()), 0, int64(payload.Len()))
	br, err := NewBlockReader(full, bh, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	buf := make([]byte, blockSize+blockSize/2)
	n, err := io.ReadFull(br, buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) {
		t.Fatalf("read %d want %d", n, len(buf))
	}
	if !bytes.Equal(buf[:blockSize], p1) {
		t.Fatal("block1 mismatch")
	}
	if !bytes.Equal(buf[blockSize:], p2) {
		t.Fatal("block2 mismatch")
	}
}
