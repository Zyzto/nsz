package keys

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_InvalidLineIgnored(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "prod.keys")
	if err := os.WriteFile(p, []byte(`
# comment
not_a_key_line
aes_kek_generation_source = 00000000000000000000000000000000
`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewStore()
	_, err := s.Load(p)
	if err == nil {
		t.Fatal("expected error: missing required keys")
	}
}

func TestAESECB_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 16)
	plain := bytes.Repeat([]byte{0xAB}, 16)
	enc, err := aesECBEncryptBlock(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := aesECBDecryptBlock(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatalf("ecb round trip failed")
	}
}
