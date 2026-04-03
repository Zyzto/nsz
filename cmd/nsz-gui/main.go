// Command nsz-gui is the GUI binary (Fyne); it can also run non-interactive operations via flags.
//
// Build or run the whole package (not main.go alone):
//
//	go run ./cmd/nsz-gui
//
// or from this directory:
//
//	go run .
package main

import (
	"context"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/zyzto/nsz/internal/core"
)

func main() {
	a := app.NewWithID("io.github.zyzto.nsz.gui")
	a.Settings().SetTheme(theme.DarkTheme())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g := newGalaxy(ctx)

	w := a.NewWindow("NSZ GUI")
	w.Resize(fyne.NewSize(820, 560))
	w.SetMaster()
	pref := a.Preferences()
	if pref.BoolWithFallback("window_fullscreen", false) {
		w.SetFullScreen(true)
	}

	queueData := binding.NewStringList()

	queueCount := widget.NewLabel("Queue: 0 items")
	updateQueueCount := func() {
		p, err := queueData.Get()
		if err != nil {
			logBindError("queueData.Get count", err)
			return
		}
		n := len(p)
		switch n {
		case 0:
			queueCount.SetText("Queue: empty")
		case 1:
			queueCount.SetText("Queue: 1 item")
		default:
			queueCount.SetText(fmt.Sprintf("Queue: %d items", n))
		}
	}
	queueData.AddListener(binding.NewDataListener(updateQueueCount))
	updateQueueCount()

	queueList := widget.NewListWithData(queueData,
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Wrapping = fyne.TextWrapWord
			rm := widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), nil)
			rm.Importance = widget.LowImportance
			return container.NewHBox(rm, lbl)
		},
		func(item binding.DataItem, o fyne.CanvasObject) {
			box, boxOK := o.(*fyne.Container)
			if !boxOK || len(box.Objects) < 2 {
				return
			}
			rm, rmOK := box.Objects[0].(*widget.Button)
			if !rmOK {
				return
			}
			lbl, lblOK := box.Objects[1].(*widget.Label)
			if !lblOK {
				return
			}
			str, strOK := item.(binding.String)
			if !strOK {
				return
			}
			lbl.Bind(str)
			rm.OnTapped = func() {
				s, err := str.Get()
				if err != nil {
					return
				}
				logBindError("queueData.Remove", queueData.Remove(s))
			}
		},
	)

	selectedQueueIdx := -1
	var removeSelectedBtn *widget.Button
	removeSelectedBtn = widget.NewButton("Remove selected", func() {
		if selectedQueueIdx < 0 {
			return
		}
		items, err := queueData.Get()
		if err != nil || selectedQueueIdx >= len(items) {
			removeSelectedBtn.Disable()
			selectedQueueIdx = -1
			return
		}
		removeQueueIndex(queueData, selectedQueueIdx)
		queueList.UnselectAll()
		removeSelectedBtn.Disable()
		selectedQueueIdx = -1
	})
	removeSelectedBtn.Disable()

	clearQueueBtn := widget.NewButton("Clear queue", func() {
		items, err := queueData.Get()
		if err != nil {
			logBindError("queueData.Get clear", err)
			return
		}
		if len(items) == 0 {
			return
		}
		dialog.ShowConfirm("Clear queue", fmt.Sprintf("Remove all %d path(s) from the queue?", len(items)), func(ok bool) {
			if !ok {
				return
			}
			logBindError("queueData.Set clear", queueData.Set([]string{}))
			queueList.UnselectAll()
			removeSelectedBtn.Disable()
			selectedQueueIdx = -1
		}, w)
	})

	queueList.OnSelected = func(id widget.ListItemID) {
		selectedQueueIdx = id
		removeSelectedBtn.Enable()
	}
	queueList.OnUnselected = func(_ widget.ListItemID) {
		selectedQueueIdx = -1
		removeSelectedBtn.Disable()
	}

	queueToolbar := container.NewHBox(
		widget.NewLabel("Select a row to remove ·"),
		removeSelectedBtn,
		clearQueueBtn,
	)

	outEntry := widget.NewEntry()
	outEntry.SetPlaceHolder("Output folder (optional)")
	if s := pref.String("output_dir"); s != "" {
		outEntry.SetText(s)
	}

	statusStr := binding.NewString()
	logBindError("statusStr initial", statusStr.Set("Ready."))
	status := widget.NewLabelWithData(statusStr)
	status.Wrapping = fyne.TextWrapWord

	progBind := binding.NewFloat()
	prog := widget.NewProgressBarWithData(progBind)
	prog.Hide()

	repCh := make(chan string, 64)
	progCh := make(chan float64, 8)
	var cancelRun context.CancelFunc

	flushStatus := func() {
		for {
			select {
			case m := <-repCh:
				logBindError("statusStr flush", statusStr.Set(m))
			default:
				return
			}
		}
	}

	runWithRep := func(opt core.Options, done func()) {
		logBindError("progBind reset", progBind.Set(0))
		prog.Show()
		ctxRun, cancel := context.WithCancel(context.Background())
		cancelRun = cancel
		go func() {
			defer done()
			rep := &chanReporter{
				info:  func(msg string) { repCh <- msg },
				warn:  func(msg string) { repCh <- msg },
				errFn: func(msg string) { repCh <- msg },
				prog: func(r, _, t int64, step string) {
					if t > 0 {
						repCh <- fmt.Sprintf("%s %d%%", step, r*100/t)
						p := float64(r) / float64(t)
						if p > 1 {
							p = 1
						}
						select {
						case progCh <- p:
						default:
						}
					} else {
						repCh <- step
					}
				},
			}
			err := core.Run(ctxRun, opt, rep)
			if err != nil {
				repCh <- "Error: " + err.Error()
			} else {
				repCh <- "Done."
			}
			select {
			case progCh <- 1:
			default:
			}
		}()
	}

	addFiles := widget.NewButton("Add files…", func() {
		d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer func() {
				if cerr := uc.Close(); cerr != nil {
					fyne.LogError("file dialog close", cerr)
				}
			}()
			p := filepath.Clean(uc.URI().Path())
			queueMergeUnique(queueData, []string{p})
		}, w)
		d.Show()
	})
	addFiles.Importance = widget.HighImportance

	scanFolder := widget.NewButton("Scan folder…", func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			added := collectArchives(uri.Path())
			if len(added) == 0 {
				dialog.ShowInformation("NSZ GUI", "No matching archives found in that folder.", w)
				return
			}
			queueMergeUnique(queueData, added)
		}, w).Show()
	})
	scanFolder.Importance = widget.HighImportance

	outBtn := widget.NewButton("Output folder…", func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outEntry.SetText(uri.Path())
			pref.SetString("output_dir", uri.Path())
		}, w).Show()
	})
	outBtn.Importance = widget.HighImportance

	compressBtn := widget.NewButton("Compress .nsp/.xci", func() {
		paths := queueSnapshot(queueData)
		if len(paths) == 0 {
			dialog.ShowInformation("NSZ GUI", "Add at least one file to the queue.", w)
			return
		}
		opt := optionsFromPrefs(pref)
		opt.Compress = true
		opt.Files = paths
		opt.Output = strings.TrimSpace(outEntry.Text)
		runWithRep(opt, func() {
			prog.Hide()
			cancelRun = nil
			logBindError("progBind done", progBind.Set(0))
		})
	})
	compressBtn.Importance = widget.SuccessImportance

	decompressBtn := widget.NewButton("Decompress .nsz/.xcz/.ncz", func() {
		paths := queueSnapshot(queueData)
		if len(paths) == 0 {
			dialog.ShowInformation("NSZ GUI", "Add at least one file to the queue.", w)
			return
		}
		opt := optionsFromPrefs(pref)
		opt.Decompress = true
		opt.Files = paths
		opt.Output = strings.TrimSpace(outEntry.Text)
		runWithRep(opt, func() {
			prog.Hide()
			cancelRun = nil
			logBindError("progBind done", progBind.Set(0))
		})
	})
	decompressBtn.Importance = widget.WarningImportance

	settingsBtn := widget.NewButton("Settings", func() { showSettings(w, pref) })
	settingsBtn.Importance = widget.LowImportance

	aboutBtn := widget.NewButton("About", func() { showAbout(w) })
	aboutBtn.Importance = widget.LowImportance

	verifyBtn := widget.NewButton("Verify", func() {
		paths := queueSnapshot(queueData)
		if len(paths) == 0 {
			dialog.ShowInformation("NSZ GUI", "Add at least one path to the queue.", w)
			return
		}
		opt := optionsFromPrefs(pref)
		opt.Files = paths
		opt.Output = strings.TrimSpace(outEntry.Text)
		opt.Verify = true
		switch pref.StringWithFallback("verify_mode", "off") {
		case "full":
			opt.QuickVerify = false
		default:
			opt.QuickVerify = true
		}
		runWithRep(opt, func() {
			prog.Hide()
			cancelRun = nil
			logBindError("progBind done", progBind.Set(0))
		})
	})

	infoBtn := widget.NewButton("Info", func() {
		paths := queueSnapshot(queueData)
		if len(paths) == 0 {
			dialog.ShowInformation("NSZ GUI", "Add at least one path to the queue.", w)
			return
		}
		opt := optionsFromPrefs(pref)
		opt.Info = true
		opt.Files = paths
		opt.Depth = pref.IntWithFallback("info_depth", 1)
		runWithRep(opt, func() {
			prog.Hide()
			cancelRun = nil
			logBindError("progBind done", progBind.Set(0))
		})
	})

	extractBtn := widget.NewButton("Extract", func() {
		paths := queueSnapshot(queueData)
		if len(paths) == 0 {
			dialog.ShowInformation("NSZ GUI", "Add at least one path to the queue.", w)
			return
		}
		opt := optionsFromPrefs(pref)
		opt.Extract = true
		opt.Files = paths
		opt.Output = strings.TrimSpace(outEntry.Text)
		opt.Depth = pref.IntWithFallback("info_depth", 1)
		opt.ExtractRegex = pref.StringWithFallback("extract_regex", "")
		runWithRep(opt, func() {
			prog.Hide()
			cancelRun = nil
			logBindError("progBind done", progBind.Set(0))
		})
	})
	extractBtn.Importance = widget.DangerImportance

	titlekeyBtn := widget.NewButton("Dump keys list", func() {
		paths := queueSnapshot(queueData)
		if len(paths) == 0 {
			dialog.ShowInformation("NSZ GUI", "Add at least one path to the queue.", w)
			return
		}
		opt := optionsFromPrefs(pref)
		opt.Titlekeys = true
		opt.Files = paths
		runWithRep(opt, func() {
			prog.Hide()
			cancelRun = nil
			logBindError("progBind done", progBind.Set(0))
		})
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		if cancelRun != nil {
			cancelRun()
			logBindError("statusStr cancel", statusStr.Set("Cancelled."))
		}
	})

	row1 := container.NewHBox(compressBtn, decompressBtn, addFiles, scanFolder, outBtn)
	row2 := container.NewHBox(settingsBtn, aboutBtn, verifyBtn, infoBtn, extractBtn, titlekeyBtn, cancelBtn)

	panel := canvas.NewRectangle(color.NRGBA{R: 12, G: 10, B: 22, A: 210})
	panel.CornerRadius = 12

	queueBody := container.NewVBox(
		queueCount,
		queueList,
		widget.NewSeparator(),
		queueToolbar,
	)
	fileCard := widget.NewCard("Queue", "Add files, scan folder, or drop onto the window. Each row has Remove; use the toolbar for selected row or clear all.", queueBody)

	center := container.NewVBox(
		widget.NewLabel("Output directory"),
		outEntry,
		fileCard,
		prog,
		status,
	)
	padded := container.NewPadded(center)
	foreground := container.NewStack(panel, padded)
	mainCol := container.NewVBox(row1, row2, foreground)

	root := container.NewStack(g.object(), mainCol)
	w.SetContent(root)

	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		mergeDroppedPaths(queueData, uris)
	})

	go func() {
		for {
			select {
			case m := <-repCh:
				logBindError("statusStr async", statusStr.Set(m))
			case p := <-progCh:
				logBindError("progBind", progBind.Set(p))
			}
		}
	}()

	w.SetCloseIntercept(func() {
		flushStatus()
		if cancelRun != nil {
			cancelRun()
		}
		cancel()
		w.Close()
	})

	w.ShowAndRun()
}

func optionsFromPrefs(pref fyne.Preferences) core.Options {
	o := core.DefaultOptions()
	fillFromPrefs(pref, &o)
	return o
}

func fillFromPrefs(pref fyne.Preferences, o *core.Options) {
	o.Level = pref.IntWithFallback("level", 18)
	if o.Level < 1 {
		o.Level = 1
	}
	if o.Level > 22 {
		o.Level = 22
	}
	o.Block = pref.BoolWithFallback("use_block", false)
	o.Solid = pref.BoolWithFallback("use_solid", false)
	o.Long = pref.BoolWithFallback("use_long", false)
	o.BlockSize = pref.IntWithFallback("block_size_exp", 20)
	o.Threads = pref.IntWithFallback("threads", -1)
	o.Multi = pref.IntWithFallback("multi", 4)
	o.Keep = pref.BoolWithFallback("keep_all", false)
	o.FixPadding = pref.BoolWithFallback("fix_padding", false)
	o.ParseCnmt = pref.BoolWithFallback("parse_cnmt", false)
	o.AlwaysParseCnmt = pref.BoolWithFallback("always_parse_cnmt", false)
	o.Overwrite = pref.BoolWithFallback("overwrite", false)
	o.RmOldVersion = pref.BoolWithFallback("rm_old_version", false)
	o.RmSource = pref.BoolWithFallback("rm_source", false)
	o.Depth = pref.IntWithFallback("info_depth", 1)
	o.ExtractRegex = pref.StringWithFallback("extract_regex", "")
	applyVerifyMode(pref, o)
}

func applyVerifyMode(pref fyne.Preferences, o *core.Options) {
	switch pref.StringWithFallback("verify_mode", "off") {
	case "quick":
		o.Verify = true
		o.QuickVerify = true
	case "full":
		o.Verify = true
		o.QuickVerify = false
	default:
		o.Verify = false
		o.QuickVerify = false
	}
}

func queueSnapshot(q binding.StringList) []string {
	p, err := q.Get()
	if err != nil || len(p) == 0 {
		return nil
	}
	out := make([]string, len(p))
	copy(out, p)
	return out
}

func queueMergeUnique(q binding.StringList, add []string) {
	if len(add) == 0 {
		return
	}
	cur, err := q.Get()
	if err != nil {
		cur = nil
	}
	seen := make(map[string]struct{}, len(cur)+len(add))
	for _, p := range cur {
		seen[p] = struct{}{}
	}
	next := append([]string(nil), cur...)
	for _, path := range add {
		path = filepath.Clean(path)
		if path == "" || path == "." {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		next = append(next, path)
	}
	if len(next) == len(cur) {
		return
	}
	logBindError("queueMergeUnique Set", q.Set(next))
}

func removeQueueIndex(q binding.StringList, i int) {
	items, err := q.Get()
	if err != nil || i < 0 || i >= len(items) {
		return
	}
	next := make([]string, 0, len(items)-1)
	next = append(next, items[:i]...)
	next = append(next, items[i+1:]...)
	logBindError("removeQueueIndex Set", q.Set(next))
}

// mergeDroppedPaths appends file:// drops to the queue. Directories are scanned
// for the same archive extensions as “Scan folder…”.
func mergeDroppedPaths(q binding.StringList, uris []fyne.URI) {
	if len(uris) == 0 {
		return
	}
	var toAdd []string
	for _, u := range uris {
		if u == nil || u.Scheme() != "file" {
			continue
		}
		p := filepath.Clean(u.Path())
		if p == "" || p == "." {
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.IsDir() {
			toAdd = append(toAdd, collectArchives(p)...)
		} else {
			toAdd = append(toAdd, p)
		}
	}
	queueMergeUnique(q, toAdd)
}

func collectArchives(root string) []string {
	var out []string
	seen := map[string]struct{}{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".nsz", ".xcz", ".ncz", ".nsp", ".xci":
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				out = append(out, path)
			}
		}
		return nil
	})
	return out
}

type chanReporter struct {
	info, warn, errFn func(string)
	prog              func(read, written, total int64, step string)
}

func (c *chanReporter) Info(msg string)  { c.info(msg) }
func (c *chanReporter) Warn(msg string)  { c.warn(msg) }
func (c *chanReporter) Error(msg string) { c.errFn(msg) }
func (c *chanReporter) Progress(r, wri, t int64, step string) {
	if c.prog != nil {
		c.prog(r, wri, t, step)
	}
}
