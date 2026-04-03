package core

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zyzto/nsz/internal/pfs0"
	"github.com/zyzto/nsz/internal/ticket"
)

type titleKeyRec struct {
	rightsID32 string
	titleKey32 string
	name       string
}

// runTitlekeys mirrors nsz ExtractTitlekeys.py (titlekeys.txt merge; titledb JSON not ported).
func runTitlekeys(ctx context.Context, opt Options, rep Reporter) error {
	if rep == nil {
		rep = NopReporter{}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	outPath := filepath.Join(cwd, "titlekeys.txt")

	dict := make(map[string]titleKeyRec)
	if st, err := os.Stat(outPath); err == nil && !st.IsDir() {
		f, err := os.Open(outPath)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) < 3 {
				continue
			}
			rid := strings.TrimSpace(parts[0])
			if len(rid) < 16 {
				continue
			}
			tid := strings.ToLower(rid[:16])
			dict[tid] = titleKeyRec{
				rightsID32: strings.ToLower(strings.TrimSpace(parts[0])),
				titleKey32: strings.ToLower(strings.TrimSpace(parts[1])),
				name:       strings.TrimSpace(parts[2]),
			}
		}
		_ = f.Close()
	}

	var sawPath, triedNspNsz bool
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
			sawPath = true
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".nsp" && ext != ".nsz" {
				rep.Info(fmt.Sprintf("skip titlekeys (need .nsp/.nsz): %s", p))
				continue
			}
			triedNspNsz = true
			pr, err := pfs0.OpenPFS0(p)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			var tik *pfs0.Entry
			for i := range pr.Entries {
				if strings.EqualFold(filepath.Ext(pr.Entries[i].Name), ".tik") {
					tik = &pr.Entries[i]
					break
				}
			}
			if tik == nil {
				rep.Info(fmt.Sprintf("Skipped ticketless %s", strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))))
				_ = pr.Close()
				continue
			}
			sec, err := pr.OpenSection(*tik)
			if err != nil {
				_ = pr.Close()
				return fmt.Errorf("%s: %w", p, err)
			}
			if _, err := sec.Seek(0, io.SeekStart); err != nil {
				_ = pr.Close()
				return fmt.Errorf("%s: %w", p, err)
			}
			raw, err := io.ReadAll(sec)
			_ = pr.Close()
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			parsed, err := ticket.Parse(raw)
			if err != nil {
				return fmt.Errorf("%s ticket: %w", p, err)
			}
			titleID := strings.ToLower(parsed.RightsID32[:16])
			stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			if _, ok := dict[titleID]; ok {
				rep.Info(fmt.Sprintf("Skipped already existing %s", parsed.RightsID32))
				continue
			}
			dict[titleID] = titleKeyRec{
				rightsID32: strings.ToLower(parsed.RightsID32),
				titleKey32: strings.ToLower(parsed.TitleKey32),
				name:       stem,
			}
			rep.Info(fmt.Sprintf("Found: %s|%s|%s", parsed.RightsID32, parsed.TitleKey32, stem))
		}
	}

	if sawPath && !triedNspNsz {
		return fmt.Errorf("titlekeys: no .nsp or .nsz in input")
	}

	ids := make([]string, 0, len(dict))
	for id := range dict {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	w, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close() }()
	rep.Info("\ntitlekeys.txt:")
	for _, id := range ids {
		rec := dict[id]
		line := fmt.Sprintf("%s|%s|%s", rec.rightsID32, rec.titleKey32, rec.name)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		rep.Info(line)
	}
	if fi, err := os.Stat(filepath.Join(cwd, "titledb")); err == nil && fi.IsDir() {
		rep.Warn("titledb/: JSON merge is not implemented in the Go port (Python updates per-title JSON files).")
	}
	return nil
}
