package aesutil

import (
	"bytes"
	"testing"
)

// NewCTRFromNonce matches PyCrypto Counter.new(64, prefix=nonce[0:8], initial_value=offset>>4).
func TestCTRStreamAtOffset(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 16)
	nonce := bytes.Repeat([]byte{7}, 16)
	offset := int64(0x4000)
	plain := []byte("hello-ctr-stream-payload-xyz")
	st, err := NewCTRFromNonce(key, nonce, offset)
	if err != nil {
		t.Fatal(err)
	}
	cipher := make([]byte, len(plain))
	st.XORKeyStream(cipher, plain)

	st2, err := NewCTRFromNonce(key, nonce, offset)
	if err != nil {
		t.Fatal(err)
	}
	again := make([]byte, len(cipher))
	st2.XORKeyStream(again, cipher)
	if !bytes.Equal(again, plain) {
		t.Fatal("round-trip XOR failed")
	}
}
