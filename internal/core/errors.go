package core

import "errors"

var (
	// ErrCompressNotImplemented is returned until the Go solid/block compressor reaches parity.
	ErrCompressNotImplemented = errors.New("nsz: compress (-C) is not implemented yet")
)
