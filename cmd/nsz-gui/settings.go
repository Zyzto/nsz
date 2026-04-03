package main

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func showSettings(parent fyne.Window, pref fyne.Preferences) {
	levelOpts := make([]string, 22)
	for i := range levelOpts {
		levelOpts[i] = strconv.Itoa(i + 1)
	}
	levelSel := widget.NewSelect(levelOpts, func(s string) {
		n, err := strconv.Atoi(s)
		if err != nil {
			fyne.LogError("settings compression level", err)
			return
		}
		pref.SetInt("level", n)
	})
	curLevel := pref.IntWithFallback("level", 18)
	if curLevel < 1 || curLevel > 22 {
		curLevel = 18
	}
	levelSel.SetSelected(strconv.Itoa(curLevel))

	blockChk := widget.NewCheck("Prefer block compression for .nsz-style output (when compression is available)", func(v bool) {
		pref.SetBool("use_block", v)
	})
	blockChk.SetChecked(pref.BoolWithFallback("use_block", false))

	solidChk := widget.NewCheck("Prefer solid compression for .xcz-style output (when compression is available)", func(v bool) {
		pref.SetBool("use_solid", v)
	})
	solidChk.SetChecked(pref.BoolWithFallback("use_solid", false))

	blockSizeLabels, blockSizeVals := blockSizeChoices()
	bsSel := widget.NewSelect(blockSizeLabels, func(s string) {
		for i, lbl := range blockSizeLabels {
			if lbl == s {
				pref.SetInt("block_size_exp", blockSizeVals[i])
				return
			}
		}
	})
	bsExp := pref.IntWithFallback("block_size_exp", 20)
	bsSel.SetSelected(blockSizeLabelForExp(blockSizeLabels, blockSizeVals, bsExp))

	verifySel := widget.NewRadioGroup([]string{"Off", "Quick (.nca hashes only)", "Full verify"}, func(s string) {
		switch s {
		case "Quick (.nca hashes only)":
			pref.SetString("verify_mode", "quick")
		case "Full verify":
			pref.SetString("verify_mode", "full")
		default:
			pref.SetString("verify_mode", "off")
		}
	})
	switch pref.StringWithFallback("verify_mode", "off") {
	case "quick":
		verifySel.SetSelected("Quick (.nca hashes only)")
	case "full":
		verifySel.SetSelected("Full verify")
	default:
		verifySel.SetSelected("Off")
	}

	keepChk := widget.NewCheck("Keep redundant data when compressing (bit-identical rebuild)", func(v bool) {
		pref.SetBool("keep_all", v)
	})
	keepChk.SetChecked(pref.BoolWithFallback("keep_all", false))

	longChk := widget.NewCheck("zstd long-distance mode (better ratio, slower)", func(v bool) {
		pref.SetBool("use_long", v)
	})
	longChk.SetChecked(pref.BoolWithFallback("use_long", false))

	compressXCI := widget.NewCheck("Enable .xci → .xcz compression (experimental; known broken in some cases)", func(v bool) {
		pref.SetBool("compress_xci", v)
	})
	compressXCI.SetChecked(pref.BoolWithFallback("compress_xci", false))

	tabSettings := container.NewScroll(settingsStack(
		settingBlock("Compression level", "Trade-off between speed and ratio. Default 18, max 22. Go: .nca→.ncz always; .xci→.xcz only if enabled below. .nsp not implemented yet.", levelSel),
		settingBlock("Block size", "Exponent for random-access blocks (2^n bytes).", bsSel),
		settingBlock("[.nsz] Block compression", "Highly parallel; better random read; slightly lower ratio.", blockChk),
		settingBlock("[.xcz] Solid compression", "Higher ratio; not suitable for mounting without decompress.", solidChk),
		settingBlock("XCI / XCZ", "Off by default. Only affects .xci inputs when using Compress.", compressXCI),
		settingBlock("Long distance mode", "", longChk),
		settingBlock("Verify", "Quick skips expensive PFS0 hash and checks .nca hashes. Full needs --keep when compressing.", verifySel),
		settingBlock("Keep everything", "", keepChk),
	))
	tabSettings.SetMinSize(fyne.NewSize(560, 420))

	threadsEnt := intEntry(pref, "threads", pref.IntWithFallback("threads", -1))
	multiEnt := intEntry(pref, "multi", pref.IntWithFallback("multi", 4))

	fixPad := widget.NewCheck("Fix PFS0 padding (common dump tool layout)", func(v bool) { pref.SetBool("fix_padding", v) })
	fixPad.SetChecked(pref.BoolWithFallback("fix_padding", false))

	parseMeta := widget.NewCheck("Parse package metadata when filename lacks id/version", func(v bool) { pref.SetBool("parse_cnmt", v) })
	parseMeta.SetChecked(pref.BoolWithFallback("parse_cnmt", false))

	alwaysMeta := widget.NewCheck("Always parse package metadata (ignore filename hints)", func(v bool) { pref.SetBool("always_parse_cnmt", v) })
	alwaysMeta.SetChecked(pref.BoolWithFallback("always_parse_cnmt", false))

	overwrite := widget.NewCheck("Overwrite existing outputs", func(v bool) { pref.SetBool("overwrite", v) })
	overwrite.SetChecked(pref.BoolWithFallback("overwrite", false))

	rmOld := widget.NewCheck("Remove older versions in output folder", func(v bool) { pref.SetBool("rm_old_version", v) })
	rmOld.SetChecked(pref.BoolWithFallback("rm_old_version", false))

	rmSrc := widget.NewCheck("Delete sources after success (use with verify)", func(v bool) { pref.SetBool("rm_source", v) })
	rmSrc.SetChecked(pref.BoolWithFallback("rm_source", false))

	tabAdvanced := container.NewScroll(settingsStack(
		settingBlock("Threads", "Compression threads; values < 1 follow tool defaults.", threadsEnt),
		settingBlock("Multitasking", "Parallel compression jobs — watch RAM at high levels.", multiEnt),
		settingBlock("Fix padding", "", fixPad),
		settingBlock("Parse metadata", "Needed for skip/overwrite and rm-old-version when names are inconsistent.", parseMeta),
		settingBlock("Always parse metadata", "", alwaysMeta),
		settingBlock("Overwrite", "", overwrite),
		settingBlock("rm-old-version", "", rmOld),
		settingBlock("rm-source", "", rmSrc),
	))
	tabAdvanced.SetMinSize(fyne.NewSize(560, 420))

	depthEnt := intEntry(pref, "info_depth", pref.IntWithFallback("info_depth", 1))
	regexEnt := widget.NewEntry()
	regexEnt.SetText(pref.StringWithFallback("extract_regex", ""))
	regexEnt.SetPlaceHolder("regular expression (optional)")
	regexEnt.OnChanged = func(s string) { pref.SetString("extract_regex", s) }

	alwaysTop := widget.NewCheck("Keep window above others (drag & drop helper)", func(v bool) {
		pref.SetBool("always_on_top", v)
	})
	alwaysTop.SetChecked(pref.BoolWithFallback("always_on_top", false))
	topNote := widget.NewLabel("Fyne does not expose always-on-top on all platforms; the choice is stored for future use.")
	topNote.Wrapping = fyne.TextWrapWord

	tabTools := container.NewScroll(settingsStack(
		settingBlock("Info depth", "Max depth for info and extraction.", depthEnt),
		settingBlock("Extract filter", "Regex for members inside a container.", regexEnt),
		settingBlock("Always on top", "", alwaysTop),
		container.NewPadded(topNote),
	))
	tabTools.SetMinSize(fyne.NewSize(560, 420))

	fullChk := widget.NewCheck("Fullscreen window", func(v bool) {
		pref.SetBool("window_fullscreen", v)
		parent.SetFullScreen(v)
	})
	fullChk.SetChecked(pref.BoolWithFallback("window_fullscreen", false))

	logLevel := widget.NewSelect([]string{"info", "warning", "error"}, func(s string) {
		pref.SetString("log_level", s)
	})
	logLevel.SetSelected(pref.StringWithFallback("log_level", "info"))

	tabWindow := container.NewScroll(settingsStack(
		settingBlock("Fullscreen", "Toggle fullscreen for this window.", fullChk),
		settingBlock("Log level", "Reserved for future file logging.", logLevel),
		container.NewPadded(widget.NewLabel("Graphics, virtual keyboard, and FPS settings from the old UI are not applicable to Fyne.")),
	))
	tabWindow.SetMinSize(fyne.NewSize(560, 420))

	tabs := container.NewAppTabs(
		container.NewTabItem("Settings", tabSettings),
		container.NewTabItem("Advanced", tabAdvanced),
		container.NewTabItem("Tools", tabTools),
		container.NewTabItem("Window", tabWindow),
	)

	closeBtn := widget.NewButton("Close", nil)

	footer := container.NewPadded(container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), closeBtn, layout.NewSpacer()),
	))

	tabsPad := container.NewPadded(tabs)
	d := widget.NewModalPopUp(
		container.NewBorder(nil, footer, nil, nil, tabsPad),
		parent.Canvas(),
	)
	closeBtn.OnTapped = func() { d.Hide() }
	// Generous size so tab scroll areas and footer do not overlap.
	d.Resize(fyne.NewSize(720, 620))
	d.Show()
}

func settingsStack(parts ...fyne.CanvasObject) fyne.CanvasObject {
	var objs []fyne.CanvasObject
	for i, p := range parts {
		if i > 0 {
			objs = append(objs, widget.NewSeparator())
		}
		objs = append(objs, p)
	}
	return container.NewVBox(objs...)
}

// settingBlock stacks title, optional description, then control full-width below.
// The old Border(right=control) layout squeezed labels and pushed controls into a narrow strip.
func settingBlock(title, desc string, ctrl fyne.CanvasObject) fyne.CanvasObject {
	t := widget.NewLabel(title)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.Wrapping = fyne.TextWrapWord
	var body []fyne.CanvasObject
	body = append(body, t)
	if strings.TrimSpace(desc) != "" {
		d := widget.NewLabel(desc)
		d.Wrapping = fyne.TextWrapWord
		body = append(body, d)
	}
	body = append(body, ctrl)
	return container.NewPadded(container.NewVBox(body...))
}

func intEntry(pref fyne.Preferences, key string, initial int) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(strconv.Itoa(initial))
	e.OnChanged = func(s string) {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return
		}
		pref.SetInt(key, n)
	}
	return e
}

func blockSizeChoices() ([]string, []int) {
	var labels []string
	var vals []int
	for exp := 14; exp <= 26; exp++ {
		sz := int64(1) << uint(exp)
		labels = append(labels, fmt.Sprintf("%s (2^%d)", humanSize(sz), exp))
		vals = append(vals, exp)
	}
	return labels, vals
}

func humanSize(n int64) string {
	const kb = 1024
	switch {
	case n >= kb*kb:
		return fmt.Sprintf("%d MB", n/(kb*kb))
	case n >= kb:
		return fmt.Sprintf("%d KB", n/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func blockSizeLabelForExp(labels []string, vals []int, exp int) string {
	for i, v := range vals {
		if v == exp {
			return labels[i]
		}
	}
	return labels[len(labels)/2]
}
