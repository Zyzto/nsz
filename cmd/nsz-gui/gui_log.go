package main

import (
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"github.com/zyzto/nsz/internal/core"
)

// guiTerminalLog writes to stderr so messages appear when the app is started from a terminal.
var guiTerminalLog = log.New(os.Stderr, "", log.LstdFlags)

var (
	progLogMu      sync.Mutex
	lastProgLog    time.Time
	progressTailRE = regexp.MustCompile(`\d+%\s*$`)
)

// logBindError logs Fyne data-binding failures (normally rare) to Fyne and stderr.
func logBindError(context string, err error) {
	if err != nil {
		fyne.LogError(context, err)
		guiTerminalLog.Printf("bind error: %s: %v", context, err)
	}
}

// guiLogStatus mirrors status-line text to stderr. Progress messages ending in "NN%"
// are throttled so the terminal is not flooded.
func guiLogStatus(msg string) {
	if shouldThrottleProgressLog(msg) {
		progLogMu.Lock()
		defer progLogMu.Unlock()
		if time.Since(lastProgLog) < 1200*time.Millisecond {
			return
		}
		lastProgLog = time.Now()
	}
	guiTerminalLog.Printf("status: %s", msg)
}

func shouldThrottleProgressLog(msg string) bool {
	msg = strings.TrimSpace(msg)
	if !strings.Contains(msg, "%") {
		return false
	}
	return progressTailRE.MatchString(msg)
}

func guiLogStartup() {
	guiTerminalLog.Printf("nsz-gui: started (pid %d); logging to stderr", os.Getpid())
}

func guiLogClose() {
	guiTerminalLog.Println("nsz-gui: window closing")
}

func guiLogCancel() {
	guiTerminalLog.Println("nsz-gui: cancel requested")
}

func guiLogQueuef(format string, args ...any) {
	guiTerminalLog.Printf("queue: "+format, args...)
}

func guiLogOutputDir(path string) {
	guiTerminalLog.Printf("output: %q", path)
}

func logCoreJobStart(opt core.Options) {
	mode := coreJobMode(opt)
	out := opt.Output
	if out == "" {
		out = "(default: next to each input)"
	}
	guiTerminalLog.Printf("job: start mode=%s files=%d output=%s level=%d threads=%d overwrite=%v compress_xci=%v",
		mode, len(opt.Files), out, opt.Level, opt.Threads, opt.Overwrite, opt.CompressXCI)
	logCoreJobFiles(opt.Files)
}

func coreJobMode(opt core.Options) string {
	switch {
	case opt.Compress:
		return "compress"
	case opt.Decompress:
		return "decompress"
	case opt.Verify:
		if opt.QuickVerify {
			return "verify-quick"
		}
		return "verify-full"
	case opt.Info:
		return "info"
	case opt.Extract:
		return "extract"
	case opt.Titlekeys:
		return "titlekeys"
	default:
		return "run"
	}
}

func logCoreJobFiles(paths []string) {
	const maxShow = 8
	if len(paths) == 0 {
		return
	}
	n := len(paths)
	if n <= maxShow {
		for _, p := range paths {
			guiTerminalLog.Printf("job:   %s", p)
		}
		return
	}
	for _, p := range paths[:maxShow] {
		guiTerminalLog.Printf("job:   %s", p)
	}
	guiTerminalLog.Printf("job:   ... +%d more", n-maxShow)
}

func guiLogCoreRunDone(err error) {
	if err != nil {
		guiTerminalLog.Printf("job: core.Run error: %v", err)
		return
	}
	guiTerminalLog.Println("job: core.Run completed OK")
}
