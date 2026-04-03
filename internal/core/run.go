package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyzto/nsz/internal/ncz"
)

// Run executes options (compress/decompress/etc.).
func Run(ctx context.Context, opt Options, rep Reporter) error {
	if rep == nil {
		rep = NopReporter{}
	}
	if opt.Compress {
		return ErrCompressNotImplemented
	}
	if opt.Output != "" {
		st, err := os.Stat(opt.Output)
		if err != nil || !st.IsDir() {
			rep.Error(fmt.Sprintf("output directory does not exist: %s", opt.Output))
			return fmt.Errorf("invalid output directory")
		}
	}
	if opt.Decompress {
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
				ext := strings.ToLower(filepath.Ext(p))
				if ext != ".nsz" && ext != ".xcz" && ext != ".ncz" {
					rep.Info(fmt.Sprintf("skip (not compressed container): %s", p))
					continue
				}
				outDir := opt.Output
				if outDir == "" {
					outDir = filepath.Dir(p)
				}
				var outPath string
				switch ext {
				case ".nsz":
					outPath = filepath.Join(outDir, strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))+".nsp")
				case ".xcz":
					rep.Warn("xcz: not implemented in Go port yet")
					continue
				case ".ncz":
					outPath = filepath.Join(outDir, strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))+".nca")
					if err := decompressNCZFile(ctx, p, outPath, rep); err != nil {
						return err
					}
					continue
				}
				if err := DecompressNSZ(ctx, p, outPath, opt.FixPadding, rep); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
			}
		}
		return nil
	}
	if opt.Info || opt.Extract || opt.Titlekeys || opt.Verify {
		return fmt.Errorf("nsz: this operation is not implemented in the Go port yet")
	}
	if len(opt.Files) > 0 {
		rep.Info("no operation flag (-C/-D/-i/...) given; try nsz --help")
	}
	return nil
}

func decompressNCZFile(ctx context.Context, srcPath, outPath string, rep Reporter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	prog := func(r, d, t int64, step string) {
		if rep != nil {
			rep.Progress(r, d, t, step)
		}
	}
	if _, _, err := ncz.Decompress(f, out, prog); err != nil {
		return err
	}
	if rep != nil {
		rep.Info(fmt.Sprintf("Wrote %s", outPath))
	}
	return out.Sync()
}
