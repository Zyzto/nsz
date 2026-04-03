package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyzto/nsz/internal/ncz"
	"github.com/zyzto/nsz/internal/pfs0"
)

// DecompressNSZ converts one .nsz to .nsp (Python __decompressNsz write path).
func DecompressNSZ(ctx context.Context, srcPath, outPath string, fixPadding bool, rep Reporter) error {
	pr, err := pfs0.OpenPFS0(srcPath)
	if err != nil {
		return err
	}
	defer pr.Close()

	var headerSize int64
	if fixPadding {
		var names []string
		for _, e := range pr.Entries {
			n := outputName(e.Name)
			names = append(names, n)
		}
		headerSize = pfs0.PaddedHeaderSize(names)
	} else {
		if len(pr.Entries) == 0 {
			return fmt.Errorf("pfs0: empty container")
		}
		headerSize = pr.Entries[0].Offset
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := out.Write(make([]byte, headerSize)); err != nil {
		return err
	}

	var recs []pfs0.FileRec
	pos := headerSize

	for _, e := range pr.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		sec, err := pr.OpenSection(e)
		if err != nil {
			return err
		}
		nameOut := outputName(e.Name)
		start := pos
		if _, err := out.Seek(pos, io.SeekStart); err != nil {
			return err
		}

		switch strings.ToLower(filepath.Ext(e.Name)) {
		case ".ncz":
			if _, err := sec.Seek(0, io.SeekStart); err != nil {
				return err
			}
			prog := func(r, d, t int64, step string) {
				if rep != nil {
					rep.Progress(r, d, t, step)
				}
			}
			if _, _, err := ncz.Decompress(sec, out, prog); err != nil {
				return fmt.Errorf("%s: %w", e.Name, err)
			}
		default:
			if _, err := sec.Seek(0, io.SeekStart); err != nil {
				return err
			}
			if _, err := io.CopyN(out, sec, e.Size); err != nil {
				return err
			}
		}
		end, err := out.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		recs = append(recs, pfs0.FileRec{Name: nameOut, Offset: start, Size: end - start})
		pos = end
	}

	hdr, err := pfs0.BuildHeaderExact(headerSize, recs)
	if err != nil {
		return err
	}
	if _, err := out.WriteAt(hdr, 0); err != nil {
		return err
	}
	if rep != nil {
		rep.Info(fmt.Sprintf("Wrote %s", outPath))
	}
	return out.Sync()
}

func outputName(name string) string {
	if strings.EqualFold(filepath.Ext(name), ".ncz") {
		return strings.TrimSuffix(name, filepath.Ext(name)) + ".nca"
	}
	return name
}
