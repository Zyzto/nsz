package main

import (
	_ "embed"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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
	linksPad := container.NewPadded(links)

	// Label uses normal foreground; disabled MultiLineEntry was low-contrast.
	lic := widget.NewLabel(embeddedLicense)
	lic.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(lic)
	scroll.SetMinSize(fyne.NewSize(560, 320))

	header := widget.NewLabelWithStyle("About NSZ GUI", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	body := container.NewVBox(
		container.NewPadded(header),
		linksPad,
		widget.NewSeparator(),
		container.NewPadded(scroll),
	)

	closeBtn := widget.NewButton("Close", nil)
	footer := container.NewPadded(container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), closeBtn, layout.NewSpacer()),
	))

	center := container.NewPadded(body)
	d := widget.NewModalPopUp(
		container.NewBorder(nil, footer, nil, nil, center),
		parent.Canvas(),
	)
	closeBtn.OnTapped = func() { d.Hide() }
	d.Resize(fyne.NewSize(640, 520))
	d.Show()
}
