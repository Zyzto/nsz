package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyzto/nsz/internal/hfs0"
	"github.com/zyzto/nsz/internal/ncz"
	"github.com/zyzto/nsz/internal/xci"
)

// Root HFS0 in XCZ output starts here (upstream XciStream).
const xczHFS0OutOffset = 0xF000

func compressXCIFile(ctx context.Context, srcPath string, opt Options, rep Reporter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	sz := fi.Size()

	xh, err := xci.ParseOpened(f)
	if err != nil {
		return fmt.Errorf("xci: %w", err)
	}
	rootEntries, _, err := hfs0.ParseAt(f, xh.HFS0Offset, sz-xh.HFS0Offset)
	if err != nil {
		return fmt.Errorf("xci root hfs0: %w", err)
	}

	zopt := ncz.SolidEncodeOptions{
		Level:   opt.Level,
		Long:    opt.Long,
		Threads: threadsForCompress(opt),
	}

	buf := make([]byte, streamBufSize)
	rootNames := make([]string, len(rootEntries))
	for i := range rootEntries {
		rootNames[i] = rootEntries[i].Name
	}
	outerHS := hfs0.HeaderLen(rootNames)
	if outerHS > hfs0.DataStart {
		return fmt.Errorf("xci: outer HFS0 header too large")
	}

	outDir := opt.Output
	if outDir == "" {
		outDir = filepath.Dir(srcPath)
	}
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	outPath := filepath.Join(outDir, base+".xcz")
	if _, err := os.Stat(outPath); err == nil && !opt.Overwrite {
		rep.Info(fmt.Sprintf("skip (exists, use -w): %s", outPath))
		return nil
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := out.Write(xh.First0x200); err != nil {
		_ = os.Remove(outPath)
		return err
	}
	pad0 := int64(xczHFS0OutOffset) - int64(len(xh.First0x200))
	if pad0 < 0 {
		_ = os.Remove(outPath)
		return fmt.Errorf("xci: first slice longer than 0xF000")
	}
	if err := writeZeros(out, pad0); err != nil {
		_ = os.Remove(outPath)
		return err
	}

	gap := hfs0.DataStart - outerHS
	if gap < 0 {
		_ = os.Remove(outPath)
		return fmt.Errorf("xci: outer header/data layout")
	}
	if _, err := out.Seek(int64(xczHFS0OutOffset)+outerHS, io.SeekStart); err != nil {
		_ = os.Remove(outPath)
		return err
	}
	if err := writeZeros(out, gap); err != nil {
		_ = os.Remove(outPath)
		return err
	}

	outerDataBase := int64(xczHFS0OutOffset) + int64(hfs0.DataStart)
	var cursor int64 = outerDataBase
	var outerToc []hfs0.TocFile

	for _, e := range rootEntries {
		if err := ctx.Err(); err != nil {
			_ = os.Remove(outPath)
			return err
		}
		if e.Size < 0 || e.Offset+e.Size > sz {
			_ = os.Remove(outPath)
			return fmt.Errorf("xci: bad partition %q", e.Name)
		}
		rel := cursor - (int64(xczHFS0OutOffset) + outerHS)
		if _, err := out.Seek(cursor, io.SeekStart); err != nil {
			_ = os.Remove(outPath)
			return err
		}
		var raw int64
		if strings.EqualFold(e.Name, "secure") {
			tmpPath, n, err := rebuildSecureToTemp(ctx, f, e.Offset, e.Size, zopt, rep, buf)
			if err != nil {
				_ = os.Remove(outPath)
				return fmt.Errorf("secure partition: %w", err)
			}
			tfin, err := os.Open(tmpPath)
			if err != nil {
				_ = os.Remove(outPath)
				_ = os.Remove(tmpPath)
				return err
			}
			copied, err := io.CopyBuffer(out, tfin, buf)
			tfin.Close()
			_ = os.Remove(tmpPath)
			if err != nil {
				_ = os.Remove(outPath)
				return err
			}
			if copied != n {
				_ = os.Remove(outPath)
				return fmt.Errorf("secure: short copy")
			}
			raw = copied
		} else {
			sr := io.NewSectionReader(f, e.Offset, e.Size)
			copied, err := io.CopyBuffer(out, sr, buf)
			if err != nil {
				_ = os.Remove(outPath)
				return err
			}
			if copied != e.Size {
				_ = os.Remove(outPath)
				return fmt.Errorf("partition %q: short copy", e.Name)
			}
			raw = copied
		}
		aligned := hfs0.Align200(raw)
		pad := aligned - raw
		if err := writeZeros(out, pad); err != nil {
			_ = os.Remove(outPath)
			return err
		}
		outerToc = append(outerToc, hfs0.TocFile{Name: e.Name, RelOff: rel, Size: aligned})
		cursor += aligned
	}

	hdr, err := hfs0.BuildHeaderBytes(outerToc)
	if err != nil {
		_ = os.Remove(outPath)
		return err
	}
	if _, err := out.WriteAt(hdr, int64(xczHFS0OutOffset)); err != nil {
		_ = os.Remove(outPath)
		return err
	}
	if err := out.Truncate(cursor); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	rep.Info(fmt.Sprintf("Wrote %s", outPath))
	if opt.RmSource {
		if err := os.Remove(srcPath); err != nil {
			return fmt.Errorf("rm-source: %w", err)
		}
	}
	return nil
}

// rebuildSecureToTemp builds inner HFS0 (streaming) into a temp file; returns path, size, error.
// Caller deletes path. Size is exact file length before outer 0x200 padding.
func rebuildSecureToTemp(ctx context.Context, xci *os.File, partOff, partSize int64, zopt ncz.SolidEncodeOptions, rep Reporter, buf []byte) (tmpPath string, size int64, err error) {
	sr := io.NewSectionReader(xci, partOff, partSize)
	entries, _, err := hfs0.ParseFrom(sr, partSize)
	if err != nil {
		if rep != nil {
			rep.Warn(fmt.Sprintf("secure: not HFS0 (%v); copying partition raw to temp", err))
		}
		tf, err := os.CreateTemp("", "nsz-xci-raw-*")
		if err != nil {
			return "", 0, err
		}
		path := tf.Name()
		rsr := io.NewSectionReader(xci, partOff, partSize)
		if _, err := io.CopyBuffer(tf, rsr, buf); err != nil {
			tf.Close()
			os.Remove(path)
			return "", 0, err
		}
		if err := tf.Close(); err != nil {
			os.Remove(path)
			return "", 0, err
		}
		st, err := os.Stat(path)
		if err != nil {
			os.Remove(path)
			return "", 0, err
		}
		return path, st.Size(), nil
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name))
		if ext == ".nca" && e.Size >= int64(ncz.UncompressibleHeaderSize) {
			names[i] = strings.TrimSuffix(e.Name, filepath.Ext(e.Name)) + ".ncz"
		} else {
			names[i] = e.Name
		}
	}
	headerSize := hfs0.HeaderLen(names)
	if headerSize > hfs0.DataStart {
		return "", 0, fmt.Errorf("secure: inner header too large")
	}

	tf, err := os.CreateTemp("", "nsz-xci-sec-*")
	if err != nil {
		return "", 0, err
	}
	path := tf.Name()
	clean := true
	defer func() {
		if clean {
			tf.Close()
			os.Remove(path)
		}
	}()
	dataStart := int64(hfs0.DataStart)
	if _, err := tf.Seek(dataStart, io.SeekStart); err != nil {
		return "", 0, err
	}
	var cursor int64 = dataStart
	var toc []hfs0.TocFile

	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		rel := cursor - headerSize
		if _, err := tf.Seek(cursor, io.SeekStart); err != nil {
			return "", 0, err
		}
		ext := strings.ToLower(filepath.Ext(e.Name))
		outName := e.Name
		if ext == ".nca" && e.Size >= int64(ncz.UncompressibleHeaderSize) {
			outName = strings.TrimSuffix(e.Name, filepath.Ext(e.Name)) + ".ncz"
			ncaSR := io.NewSectionReader(xci, partOff+e.Offset, e.Size)
			if err := ncz.WriteSolidNCZFromReader(tf, ncaSR, e.Size, zopt); err != nil {
				return "", 0, fmt.Errorf("%s: %w", e.Name, err)
			}
		} else {
			// Non-NCA, or tiny .nca (e.g. CNMT) smaller than 0x4000: solid NCZ layout does not apply; copy raw.
			memSR := io.NewSectionReader(xci, partOff+e.Offset, e.Size)
			n, err := io.CopyBuffer(tf, memSR, buf)
			if err != nil {
				return "", 0, err
			}
			if n != e.Size {
				return "", 0, fmt.Errorf("%s: short copy", e.Name)
			}
		}
		end, err := tf.Seek(0, io.SeekCurrent)
		if err != nil {
			return "", 0, err
		}
		sz := end - cursor
		toc = append(toc, hfs0.TocFile{Name: outName, RelOff: rel, Size: sz})
		cursor = end
	}

	hdr, err := hfs0.BuildHeaderBytes(toc)
	if err != nil {
		return "", 0, err
	}
	if _, err := tf.WriteAt(hdr, 0); err != nil {
		return "", 0, err
	}
	if err := tf.Close(); err != nil {
		return "", 0, err
	}
	clean = false
	st, err := os.Stat(path)
	if err != nil {
		os.Remove(path)
		return "", 0, err
	}
	return path, st.Size(), nil
}
