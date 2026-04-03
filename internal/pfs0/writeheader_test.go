package pfs0

import "testing"

func TestBuildHeaderExact(t *testing.T) {
	headerSize := int64(0x100)
	files := []FileRec{
		{Name: "a.nca", Offset: headerSize, Size: 10},
		{Name: "b.bin", Offset: headerSize + 10, Size: 20},
	}
	h, err := BuildHeaderExact(headerSize, files)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(h)) != headerSize {
		t.Fatalf("len %d headerSize %d", len(h), headerSize)
	}
	if string(h[:4]) != "PFS0" {
		t.Fatal("magic")
	}
}
