package ncz

import (
	"io"
)

// DecompressedSize returns total NCA size after decompress (Python __getDecompressedNczSize).
func DecompressedSize(rs io.ReadSeeker) (int64, error) {
	ir, err := Inspect(rs)
	if err != nil {
		return 0, err
	}
	return ir.DecompressedSize, nil
}
