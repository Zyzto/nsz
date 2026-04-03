package ncz

import (
	"bytes"
	"testing"
)

func TestInspectSolid(t *testing.T) {
	want := bytes.Repeat([]byte{0xCD}, 12345)
	raw, err := buildSolidNCZ(want)
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(raw)
	ir, err := Inspect(r)
	if err != nil {
		t.Fatal(err)
	}
	if ir.BlockCompressed {
		t.Fatal("expected solid NCZ")
	}
	if ir.DecompressedSize != int64(UncompressibleHeaderSize+len(want)) {
		t.Fatalf("decompressed size %d want %d", ir.DecompressedSize, UncompressibleHeaderSize+len(want))
	}
	if len(ir.Sections) < 1 {
		t.Fatal("expected sections")
	}
}
