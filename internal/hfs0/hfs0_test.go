package hfs0

import (
	"os"
	"testing"
)

func TestBuildParseRoundTrip(t *testing.T) {
	files := []FileRec{
		{Name: "a.bin", Data: []byte{1, 2, 3}},
		{Name: "b.bin", Data: make([]byte, 500)},
	}
	built, err := Build(files)
	if err != nil {
		t.Fatal(err)
	}
	tf, err := os.CreateTemp("", "hfs0-rt-*")
	if err != nil {
		t.Fatal(err)
	}
	path := tf.Name()
	defer os.Remove(path)
	if _, err := tf.Write(built); err != nil {
		t.Fatal(err)
	}
	if err := tf.Close(); err != nil {
		t.Fatal(err)
	}
	rf, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	entries, hs, err := ParseAt(rf, 0, int64(len(built)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries %d", len(entries))
	}
	if hs <= 0 {
		t.Fatal("header")
	}
	for i, e := range entries {
		buf := make([]byte, e.Size)
		if _, err := rf.ReadAt(buf, e.Offset); err != nil {
			t.Fatal(err)
		}
		if e.Name != files[i].Name {
			t.Fatalf("name %q want %q", e.Name, files[i].Name)
		}
		if string(buf) != string(files[i].Data) {
			t.Fatalf("payload mismatch %s", e.Name)
		}
	}
}
