package ncz

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/klauspost/compress/zstd"
)

// SolidEncodeOptions configures solid NCZ zstd output (upstream NSZ default for .nsz members).
type SolidEncodeOptions struct {
	Level   int  // zstd level 1–22 (upstream default 18)
	Long    bool // use maximum encoder window (closest to Python --long / enable_ldm; framing may still differ from C zstd)
	Threads int  // encoder concurrency; 1 = deterministic stream
}

// WriteSolidNCZ writes a solid NCZ stream for an NCA whose bytes after 0x4000 form one plaintext
// section (cryptoType 1). This matches the on-disk layout Python produces when getEncryptionSections()
// collapses to that single section; arbitrary retail NCAs usually have multiple CTR sections and
// require the full upstream compressor.
func WriteSolidNCZ(w io.Writer, nca []byte, opt SolidEncodeOptions) error {
	return WriteSolidNCZFromReader(w, bytes.NewReader(nca), int64(len(nca)), opt)
}

// WriteSolidNCZFromPath streams from an NCA file (constant memory for the body).
func WriteSolidNCZFromPath(w io.Writer, ncaPath string, opt SolidEncodeOptions) error {
	f, err := os.Open(ncaPath)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	return WriteSolidNCZFromReader(w, f, st.Size(), opt)
}

// WriteSolidNCZFromReader streams solid NCZ from a ReaderAt (e.g. *os.File or SectionReader).
func WriteSolidNCZFromReader(w io.Writer, r io.ReaderAt, ncaTotal int64, opt SolidEncodeOptions) error {
	if ncaTotal < UncompressibleHeaderSize {
		return fmt.Errorf("ncz: input shorter than 0x%x bytes", UncompressibleHeaderSize)
	}
	level := opt.Level
	if level < 1 {
		level = 18
	}
	if level > 22 {
		level = 22
	}
	threads := opt.Threads
	if threads < 1 {
		threads = 3
	}
	if threads > runtime.GOMAXPROCS(0)*2 {
		threads = runtime.GOMAXPROCS(0) * 2
	}

	header := make([]byte, UncompressibleHeaderSize)
	if _, err := r.ReadAt(header, 0); err != nil {
		return err
	}
	bodySize := ncaTotal - UncompressibleHeaderSize

	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write([]byte("NCZSECTN")); err != nil {
		return err
	}
	secCount := int64(1)
	if err := binary.Write(w, binary.LittleEndian, secCount); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, int64(UncompressibleHeaderSize)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, bodySize); err != nil {
		return err
	}
	ct := int64(1) // plaintext
	if err := binary.Write(w, binary.LittleEndian, ct); err != nil {
		return err
	}
	var pad int64
	if err := binary.Write(w, binary.LittleEndian, pad); err != nil {
		return err
	}
	if _, err := w.Write(make([]byte, 32)); err != nil {
		return err
	}

	zopts := []zstd.EOption{
		zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)),
		zstd.WithEncoderConcurrency(threads),
	}
	if opt.Long {
		zopts = append(zopts, zstd.WithWindowSize(zstd.MaxWindowSize))
	}
	zw, err := zstd.NewWriter(w, zopts...)
	if err != nil {
		return err
	}
	body := io.NewSectionReader(r, int64(UncompressibleHeaderSize), bodySize)
	if _, err := io.Copy(zw, body); err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}
