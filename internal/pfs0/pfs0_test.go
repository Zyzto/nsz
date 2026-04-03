package pfs0

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestParseNames_TOCOrder builds a PFS0 where the second name comes after the first in the
// string table (normal layout). Forward-only name slicing would produce wrong/empty names.
func TestParseNames_TOCOrder(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.pfs0")

	strTable := "a.bin\x00longer.ncz\x00"
	strPad := len(strTable)
	for (0x10+2*0x18+strPad)%0x20 != 0 {
		strTable += "\x00"
		strPad++
	}
	headerSize := int64(0x10 + 2*0x18 + strPad)

	var b []byte
	b = append(b, []byte("PFS0")...)
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, 2)
	b = append(b, tmp...)
	binary.LittleEndian.PutUint32(tmp, uint32(strPad))
	b = append(b, tmp...)
	b = append(b, 0, 0, 0, 0)

	// Row 0: rel off 0, size 1, nameOff 0
	row := make([]byte, 0x18)
	binary.LittleEndian.PutUint64(row[0:8], 0)
	binary.LittleEndian.PutUint64(row[8:16], 1)
	binary.LittleEndian.PutUint32(row[16:20], 0)
	b = append(b, row...)
	// Row 1: rel off 1, size 1, nameOff 6 ("a.bin\x00")
	binary.LittleEndian.PutUint64(row[0:8], 1)
	binary.LittleEndian.PutUint64(row[8:16], 1)
	binary.LittleEndian.PutUint32(row[16:20], 6)
	b = append(b, row...)
	b = append(b, []byte(strTable)...)

	if int64(len(b)) != headerSize {
		t.Fatalf("header len %d want %d", len(b), headerSize)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}

	pr, err := OpenPFS0(p)
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	if len(pr.Entries) != 2 {
		t.Fatalf("entries %d", len(pr.Entries))
	}
	if pr.Entries[0].Name != "a.bin" || pr.Entries[1].Name != "longer.ncz" {
		t.Fatalf("names %q %q", pr.Entries[0].Name, pr.Entries[1].Name)
	}
}
