package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// nszTheme tweaks the stock dark palette so primary actions read clearly on the animated background.
type nszTheme struct {
	base fyne.Theme
}

func newNSZTheme() fyne.Theme {
	return &nszTheme{base: theme.DarkTheme()}
}

func (t *nszTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 52, G: 168, B: 112, A: 255}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 232, G: 168, B: 64, A: 255}
	case theme.ColorNameError:
		return color.NRGBA{R: 220, G: 82, B: 96, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 96, G: 132, B: 232, A: 255}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 130, G: 170, B: 255, A: 180}
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground:
		c := t.base.Color(n, v)
		if nrgba, ok := c.(color.NRGBA); ok {
			nrgba.A = 240
			return nrgba
		}
		return c
	default:
		return t.base.Color(n, v)
	}
}

func (t *nszTheme) Font(s fyne.TextStyle) fyne.Resource {
	return t.base.Font(s)
}

func (t *nszTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(n)
}

func (t *nszTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		v := t.base.Size(n)
		if v < 6 {
			return 6
		}
		return v
	case theme.SizeNameInnerPadding:
		v := t.base.Size(n)
		if v < 4 {
			return 4
		}
		return v
	default:
		return t.base.Size(n)
	}
}
