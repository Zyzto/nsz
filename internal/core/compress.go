package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyzto/nsz/internal/ncz"
)

// runCompress implements -C for formats we can emit with correct NCZ layout.
func runCompress(ctx context.Context, opt Options, rep Reporter) error {
	if rep == nil {
		rep = NopReporter{}
	}
	if opt.Block {
		rep.Warn("compress: block mode (-B) is not implemented in Go; only solid .nca → .ncz is supported")
	}
	if opt.Verify {
		rep.Warn("compress: post-compression verify (-V) is not implemented in Go yet")
	}
	if opt.Output != "" {
		st, err := os.Stat(opt.Output)
		if err != nil || !st.IsDir() {
			rep.Error(fmt.Sprintf("output directory does not exist: %s", opt.Output))
			return fmt.Errorf("invalid output directory")
		}
	}
	for _, f := range opt.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		matches, err := filepath.Glob(f)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			matches = []string{f}
		}
		for _, p := range matches {
			if err := ctx.Err(); err != nil {
				return err
			}
			ext := strings.ToLower(filepath.Ext(p))
			switch ext {
			case ".nca":
				if err := compressNCAFile(ctx, p, opt, rep); err != nil {
					return err
				}
			case ".xci":
				if !opt.CompressXCI {
					rep.Info(fmt.Sprintf("skip (.xci compression disabled; turn on in GUI settings or use -compress-xci): %s", p))
					continue
				}
				if err := compressXCIFile(ctx, p, opt, rep); err != nil {
					return err
				}
			case ".nsp":
				return fmt.Errorf("%w (.nsp needs full PFS0+NCA section pipeline like upstream NSZ)", ErrCompressContainerNotImplemented)
			default:
				rep.Info(fmt.Sprintf("skip (compress only .nca in Go port): %s", p))
			}
		}
	}
	return nil
}

func compressNCAFile(ctx context.Context, srcPath string, opt Options, rep Reporter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if st, err := os.Stat(srcPath); err != nil {
		return err
	} else if st.Size() < int64(ncz.UncompressibleHeaderSize) {
		return fmt.Errorf("%s: file too small for NCA header", srcPath)
	}
	outDir := opt.Output
	if outDir == "" {
		outDir = filepath.Dir(srcPath)
	}
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	outPath := filepath.Join(outDir, base+".ncz")
	if _, err := os.Stat(outPath); err == nil && !opt.Overwrite {
		rep.Info(fmt.Sprintf("skip (exists, use -w): %s", outPath))
		return nil
	}
	threads := opt.Threads
	if threads < 1 {
		threads = 3 // upstream NSZ default for solid compression when -t < 1
	}
	zopt := ncz.SolidEncodeOptions{
		Level:   opt.Level,
		Long:    opt.Long,
		Threads: threads,
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if err := ncz.WriteSolidNCZFromPath(out, srcPath, zopt); err != nil {
		_ = os.Remove(outPath)
		return fmt.Errorf("%s: %w", srcPath, err)
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

func threadsForCompress(opt Options) int {
	t := opt.Threads
	if t < 1 {
		return 3
	}
	return t
}
