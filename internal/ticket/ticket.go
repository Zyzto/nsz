// Package ticket parses Switch ticket (.tik) blobs (upstream nsz Fs/Ticket.py layout).
package ticket

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// Signature sizes match nsz.Fs.Type.TicketSignature.
var signatureSizes = map[uint32]int{
	0x010000: 0x200, // RSA_4096_SHA1
	0x010001: 0x100, // RSA_2048_SHA1
	0x010002: 0x3C,  // ECDSA_SHA1
	0x010003: 0x200, // RSA_4096_SHA256
	0x010004: 0x100, // RSA_2048_SHA256
	0x010005: 0x3C,  // ECDSA_SHA256
}

// Parsed holds fields used for titlekeys.txt (upstream ExtractTitlekeys / Ticket.printInfo).
type Parsed struct {
	RightsID32 string // 32 hex chars (16 bytes)
	TitleKey32 string // 32 hex chars (first 16 bytes of title key block, big-endian style)
}

// Parse extracts rights id and title key block prefix from raw ticket bytes.
func Parse(raw []byte) (*Parsed, error) {
	if len(raw) < 0x180 {
		return nil, fmt.Errorf("ticket: too short (%d)", len(raw))
	}
	sigType := binary.LittleEndian.Uint32(raw[0:4])
	sigLen, ok := signatureSizes[sigType]
	if !ok {
		return nil, fmt.Errorf("ticket: unknown signature type 0x%x", sigType)
	}
	// Match Python: 0x40 - ((sigSize + 4) % 0x40) — can be 0x40 when aligned.
	pad := 0x40 - ((sigLen + 4) % 0x40)
	base := 4 + sigLen + pad
	if base < 0 || base+0x170 > len(raw) {
		return nil, fmt.Errorf("ticket: invalid header layout")
	}
	// Layout after base: issuer 0x40, titleKeyBlock 0x100, ...
	tk := raw[base+0x40 : base+0x50]
	if len(tk) != 16 {
		return nil, fmt.Errorf("ticket: title key slice")
	}
	rid := raw[base+0x160 : base+0x170]
	if len(rid) != 16 {
		return nil, fmt.Errorf("ticket: rights id slice")
	}
	return &Parsed{
		RightsID32: hex.EncodeToString(rid),
		TitleKey32: hex.EncodeToString(tk),
	}, nil
}
