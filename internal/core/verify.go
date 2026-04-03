package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyzto/nsz/internal/ncz"
	"github.com/zyzto/nsz/internal/pfs0"
)

// runVerify checks compressed payloads (Python NszDecompressor.verify subset).
// — Standalone .ncz: full decompress+SHA256; optional filename vs hash prefix check.
// — .nsp/.nsz: each .ncz member is verified the same way.
// CNMT-listed expected hashes for plain .nca and outer PFS0 rebuild are not implemented yet
// (needs NCA/CNMT crypto stack); quick vs full both run the NCZ member checks.
func runVerify(ctx context.Context, opt Options, rep Reporter) error {
	if rep == nil {
		rep = NopReporter{}
	}
	if !opt.QuickVerify {
		rep.Warn("verify: full PFS0/container rebuild check is not implemented in Go; running NCZ member checks only (same as quick for .ncz content).")
	}
	var sawXCIish, didVerify bool
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
			case ".ncz":
				rep.Info(fmt.Sprintf("[VERIFY ncz] %s", filepath.Base(p)))
				if err := verifyNCZFile(ctx, p, rep); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				didVerify = true
			case ".nsz", ".nsp":
				rep.Info(fmt.Sprintf("[VERIFY %s] %s", strings.TrimPrefix(ext, "."), filepath.Base(p)))
				if err := verifyPFS0NCZMembers(ctx, p, rep); err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				didVerify = true
			case ".xcz", ".xci":
				sawXCIish = true
				rep.Warn(fmt.Sprintf("verify: %s is not implemented in the Go port yet", ext))
			default:
				rep.Info(fmt.Sprintf("skip verify (unsupported): %s", p))
			}
		}
	}
	if sawXCIish && !didVerify {
		return fmt.Errorf("verify: .xcz/.xci are not implemented in the Go port yet")
	}
	return nil
}

func verifyNCZFile(ctx context.Context, path string, rep Reporter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	prog := func(r, d, tot int64, step string) {
		if rep != nil {
			rep.Progress(r, d, tot, step)
		}
	}
	_, hexHash, err := ncz.Decompress(f, nil, prog)
	if err != nil {
		return err
	}
	rep.Info(fmt.Sprintf("[NCA HASH]   %s", hexHash))
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if len(hexHash) >= 32 && stem == hexHash[:32] {
		rep.Info(fmt.Sprintf("[VERIFIED]   %s", filepath.Base(path)))
	} else if len(stem) == 32 {
		rep.Error(fmt.Sprintf("[MISMATCH]   filename prefix %s vs hash %s", stem, hexHash[:32]))
		return fmt.Errorf("ncz hash mismatch with filename")
	} else {
		rep.Info(fmt.Sprintf("[VERIFIED]   %s (hash computed; filename not 32-hex)", filepath.Base(path)))
	}
	return nil
}

func verifyPFS0NCZMembers(ctx context.Context, path string, rep Reporter) error {
	pr, err := pfs0.OpenPFS0(path)
	if err != nil {
		return err
	}
	defer pr.Close()

	for _, e := range pr.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !strings.EqualFold(filepath.Ext(e.Name), ".ncz") {
			continue
		}
		rep.Info(fmt.Sprintf("[EXISTS]     %s", e.Name))
		sec, err := pr.OpenSection(e)
		if err != nil {
			return err
		}
		prog := func(r, d, tot int64, step string) {
			if rep != nil {
				rep.Progress(r, d, tot, step)
			}
		}
		_, hexHash, err := ncz.Decompress(sec, nil, prog)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
		rep.Info(fmt.Sprintf("[NCA HASH]   %s", hexHash))
		rep.Info(fmt.Sprintf("[NCZ OK]     %s (decompress+hash OK; CNMT hash list compare not implemented in Go)", e.Name))
	}
	return nil
}
