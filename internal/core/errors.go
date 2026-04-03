package core

import "errors"

var (
	// ErrCompressContainerNotImplemented is returned for .nsp/.xci until the NCA section pipeline is ported.
	ErrCompressContainerNotImplemented = errors.New("nsz: compress for this container type is not implemented in Go yet")
)
