package main

import (
	_ "embed"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

//go:embed legal/LICENSE
var embeddedLicense string

func showAbout(parent fyne.Window) {
	uMust := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			fyne.LogError("about dialog URL", err)
			return &url.URL{}
		}
		return u
	}

	links := container.NewHBox(
		widget.NewHyperlink("GitHub", uMust("https://github.com/zyzto/nsz")),
		widget.NewHyperlink("Releases", uMust("https://github.com/zyzto/nsz/releases")),
		widget.NewHyperlink("Issues", uMust("https://github.com/zyzto/nsz/issues")),
		widget.NewHyperlink("Contributors", uMust("https://github.com/zyzto/nsz/graphs/contributors")),
		widget.NewHyperlink("License", uMust("https://github.com/zyzto/nsz/blob/master/LICENSE")),
	)

	license := widget.NewMultiLineEntry()
	license.SetText(embeddedLicense)
	license.Disable()
	license.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(license)
	scroll.SetMinSize(fyne.NewSize(520, 280))

	body := container.NewVBox(
		widget.NewLabelWithStyle("About NSZ GUI", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		links,
		widget.NewSeparator(),
		scroll,
	)

	closeBtn := widget.NewButton("Close", nil)
	d := widget.NewModalPopUp(
		container.NewBorder(nil, closeBtn, nil, nil, body),
		parent.Canvas(),
	)
	closeBtn.OnTapped = func() { d.Hide() }
	d.Resize(fyne.NewSize(560, 420))
	d.Show()
}
