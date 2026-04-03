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
	"github.com/zyzto/nsz/internal/pfs0"
	"github.com/zyzto/nsz/internal/xci"
)

// runInfo implements -i / --info for formats supported in the Go port (Python printInfo subset).
func runInfo(ctx context.Context, opt Options, rep Reporter) error {
	if rep == nil {
		rep = NopReporter{}
	}
	var sawXcz, didInfo bool
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
			case ".nsp", ".nsz":
				if err := infoPFS0(ctx, rep, p, opt.Depth); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
			case ".ncz":
				if err := infoNCZPath(rep, p, opt.Depth); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
			case ".xci":
				if err := infoXCI(ctx, rep, p, opt.Depth); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
			case ".xcz":
				sawXcz = true
				rep.Warn("info: .xcz is not implemented in the Go port yet")
			default:
				rep.Info(fmt.Sprintf("skip (unsupported for -i): %s", p))
			}
			switch ext {
			case ".nsp", ".nsz", ".ncz", ".xci":
				didInfo = true
			}
		}
	}
	if sawXcz && !didInfo {
		return fmt.Errorf("info: .xcz is not implemented in the Go port yet")
	}
	return nil
}

func infoPFS0(ctx context.Context, rep Reporter, path string, depth int) error {
	pr, err := pfs0.OpenPFS0(path)
	if err != nil {
		return err
	}
	defer pr.Close()

	rep.Info(path)
	for _, e := range pr.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		rep.Info(fmt.Sprintf("  %-48s  offset=0x%x  size=%d", e.Name, e.Offset, e.Size))
		if depth >= 2 && strings.EqualFold(filepath.Ext(e.Name), ".ncz") {
			sec, err := pr.OpenSection(e)
			if err != nil {
				return err
			}
			if err := infoNCZReader(rep, "    ", sec); err != nil {
				return err
			}
		}
	}
	return nil
}

func infoNCZPath(rep Reporter, path string, depth int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	rep.Info(path)
	if depth >= 2 {
		return infoNCZReader(rep, "  ", f)
	}
	ir, err := ncz.Inspect(f)
	if err != nil {
		return err
	}
	mode := "solid"
	if ir.BlockCompressed {
		mode = "block"
	}
	rep.Info(fmt.Sprintf("  decompressed_size=%d  mode=%s  sections=%d", ir.DecompressedSize, mode, len(ir.Sections)))
	return nil
}

func infoXCI(ctx context.Context, rep Reporter, path string, depth int) error {
	f, err := os.Open(path)
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
		return err
	}
	entries, _, err := hfs0.ParseAt(f, xh.HFS0Offset, sz-xh.HFS0Offset)
	if err != nil {
		return err
	}
	rep.Info(path)
	rep.Info("  (XCI root HFS0 partitions)")
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		rep.Info(fmt.Sprintf("  %-48s  offset=0x%x  size=%d", e.Name, e.Offset, e.Size))
	}
	if depth >= 2 {
		rep.Warn("info: depth>=2 secure partition listing is not implemented for .xci in Go yet")
	}
	return nil
}

func infoNCZReader(rep Reporter, indent string, rs io.ReadSeeker) error {
	ir, err := ncz.Inspect(rs)
	if err != nil {
		return err
	}
	mode := "solid"
	if ir.BlockCompressed {
		mode = "block"
	}
	rep.Info(fmt.Sprintf("%sncz: decompressed_size=%d mode=%s sections=%d", indent, ir.DecompressedSize, mode, len(ir.Sections)))
	for i, s := range ir.Sections {
		rep.Info(fmt.Sprintf("%s  [%d] offset=0x%x size=%d cryptoType=%d", indent, i, s.Offset, s.Size, s.CryptoType))
	}
	return nil
}
