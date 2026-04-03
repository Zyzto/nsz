package core

import (
	"io"
)

// streamBufSize is used for io.CopyBuffer in compress paths (disk ↔ disk).
const streamBufSize = 512 * 1024

func writeZeros(w io.Writer, n int64) error {
	var z [8192]byte
	for n > 0 {
		k := int64(len(z))
		if k > n {
			k = n
		}
		if _, err := w.Write(z[:k]); err != nil {
			return err
		}
		n -= k
	}
	return nil
}
