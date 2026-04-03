// Command nsz is the CLI-only binary (no GUI dependencies).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/schollz/progressbar/v3"
	"github.com/zyzto/nsz/internal/core"
)

func main() {
	os.Exit(run())
}

func run() int {
	opt := core.DefaultOptions()
	flag.BoolVar(&opt.Compress, "C", false, "Compress .nsp/.xci/.nca (.xci requires -compress-xci)")
	flag.BoolVar(&opt.CompressXCI, "compress-xci", false, "Allow .xci → .xcz (experimental; off by default)")
	flag.BoolVar(&opt.Decompress, "D", false, "Decompress .nsz/.xcz/.ncz")
	flag.IntVar(&opt.Level, "l", 18, "Compression level (default 18)")
	flag.IntVar(&opt.Level, "level", 18, "Compression level")
	flag.BoolVar(&opt.Long, "L", false, "zstd long distance mode")
	flag.BoolVar(&opt.Long, "long", false, "zstd long distance mode")
	flag.BoolVar(&opt.Block, "B", false, "Block compression")
	flag.BoolVar(&opt.Block, "block", false, "Block compression")
	flag.BoolVar(&opt.Solid, "S", false, "Solid compression")
	flag.BoolVar(&opt.Solid, "solid", false, "Solid compression")
	flag.IntVar(&opt.BlockSize, "s", 20, "Block size exponent (2^x)")
	flag.IntVar(&opt.BlockSize, "bs", 20, "Block size exponent")
	flag.BoolVar(&opt.Verify, "V", false, "Verify")
	flag.BoolVar(&opt.Verify, "verify", false, "Verify")
	flag.BoolVar(&opt.QuickVerify, "Q", false, "Quick verify (.nca hashes only)")
	flag.BoolVar(&opt.QuickVerify, "quick-verify", false, "Quick verify")
	flag.BoolVar(&opt.Keep, "K", false, "Keep redundant partitions")
	flag.BoolVar(&opt.Keep, "keep", false, "Keep redundant partitions")
	flag.BoolVar(&opt.FixPadding, "F", false, "Fix PFS0 padding")
	flag.BoolVar(&opt.FixPadding, "fix-padding", false, "Fix PFS0 padding")
	flag.BoolVar(&opt.ParseCnmt, "p", false, "Parse package metadata for title id")
	flag.BoolVar(&opt.ParseCnmt, "parseCnmt", false, "Parse package metadata (same as -p)")
	flag.BoolVar(&opt.AlwaysParseCnmt, "P", false, "Always parse package metadata")
	flag.BoolVar(&opt.AlwaysParseCnmt, "alwaysParseCnmt", false, "Always parse package metadata")
	flag.IntVar(&opt.Threads, "t", -1, "Threads")
	flag.IntVar(&opt.Threads, "threads", -1, "Threads")
	flag.IntVar(&opt.Multi, "m", 4, "Parallel compression jobs")
	flag.IntVar(&opt.Multi, "multi", 4, "Parallel compression jobs")
	flag.StringVar(&opt.Output, "o", "", "Output directory")
	flag.StringVar(&opt.Output, "output", "", "Output directory")
	flag.BoolVar(&opt.Overwrite, "w", false, "Overwrite")
	flag.BoolVar(&opt.Overwrite, "overwrite", false, "Overwrite")
	flag.BoolVar(&opt.RmOldVersion, "r", false, "Remove old versions")
	flag.BoolVar(&opt.RmOldVersion, "rm-old-version", false, "Remove old versions")
	var rmSource bool
	flag.BoolVar(&rmSource, "rm-source", false, "Delete source after success")
	flag.BoolVar(&opt.Info, "i", false, "Show info")
	flag.BoolVar(&opt.Info, "info", false, "Show info")
	flag.IntVar(&opt.Depth, "depth", 1, "Info depth")
	flag.BoolVar(&opt.Extract, "x", false, "Extract container")
	flag.BoolVar(&opt.Extract, "extract", false, "Extract container")
	flag.StringVar(&opt.ExtractRegex, "extractregex", "", "Extract regex filter")
	flag.BoolVar(&opt.Titlekeys, "titlekeys", false, "Extract titlekeys")
	flag.BoolVar(&opt.Undupe, "undupe", false, "Deduplicate")
	flag.BoolVar(&opt.UndupeDryRun, "undupe-dryrun", false, "Undupe dry run")
	flag.BoolVar(&opt.UndupeRename, "undupe-rename", false, "Undupe rename")
	flag.BoolVar(&opt.UndupeHardlink, "undupe-hardlink", false, "Undupe hardlink")
	flag.StringVar(&opt.UndupePriority, "undupe-prioritylist", "", "Undupe priority regex")
	flag.StringVar(&opt.UndupeWhitelist, "undupe-whitelist", "", "Undupe whitelist regex")
	flag.StringVar(&opt.UndupeBlacklist, "undupe-blacklist", "", "Undupe blacklist regex")
	flag.BoolVar(&opt.UndupeOldVersions, "undupe-old-versions", false, "Undupe old versions")
	flag.StringVar(&opt.Create, "c", "", "Create .nsp from folder")
	flag.StringVar(&opt.Create, "create", "", "Create .nsp from folder")
	flag.BoolVar(&opt.MachineReadable, "machine-readable", false, "Machine-readable progress")
	flag.BoolVar(&opt.Verbose, "v", false, "Verbose")
	flag.BoolVar(&opt.Verbose, "verbose", false, "Verbose")
	flag.BoolVar(&opt.Quiet, "q", false, "Quiet")
	flag.BoolVar(&opt.Quiet, "quiet", false, "Quiet")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "nsz — PFS0 / NSZ family compressor-decompressor (Go port; experimental, may not work)\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Prefer upstream Python NSZ for production use unless you have verified this binary.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	opt.Files = flag.Args()
	opt.RmSource = rmSource

	if opt.QuickVerify {
		opt.Verify = true
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	rep := newTermReporter(opt)
	if err := core.Run(ctx, opt, rep); err != nil {
		if errors.Is(err, core.ErrCompressContainerNotImplemented) {
			fmt.Fprintln(os.Stderr, err)
			return 3
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type termReporter struct {
	opt       core.Options
	bar       *progressbar.ProgressBar
	lastTotal int64
}

func newTermReporter(opt core.Options) *termReporter {
	return &termReporter{opt: opt}
}

func (t *termReporter) Info(msg string) {
	if t.opt.Quiet {
		return
	}
	fmt.Println(msg)
}

func (t *termReporter) Warn(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func (t *termReporter) Error(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func (t *termReporter) Progress(readB, writtenB, total int64, step string) {
	if t.opt.MachineReadable {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"type":    "progress",
			"read":    readB,
			"written": writtenB,
			"total":   total,
			"step":    step,
			"percent": percent(readB, total),
		})
		return
	}
	if t.opt.Quiet {
		return
	}
	if total <= 0 {
		return
	}
	if t.bar == nil || t.lastTotal != total {
		t.lastTotal = total
		t.bar = progressbar.New64(total)
		t.bar.Describe(step)
	}
	_ = t.bar.Set64(readB)
}

func percent(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}
