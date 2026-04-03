package aesutil

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

// NewCTRFromNonce matches nsz nut/aes128.AESCTR seek: Counter.new(64, prefix=nonce[0:8], initial_value=offset>>4).
func NewCTRFromNonce(key, nonce []byte, offset int64) (cipher.Stream, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("aesutil: key length %d, want 16", len(key))
	}
	if len(nonce) < 8 {
		return nil, fmt.Errorf("aesutil: nonce length %d, want at least 8", len(nonce))
	}
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, 16)
	copy(iv[:8], nonce[:8])
	binary.BigEndian.PutUint64(iv[8:], uint64(offset>>4))
	return cipher.NewCTR(c, iv), nil
}

// XOR applies stream to dst/src.
func XOR(stream cipher.Stream, dst, src []byte) {
	stream.XORKeyStream(dst, src)
}
