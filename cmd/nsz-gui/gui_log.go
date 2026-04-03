package main

import "fyne.io/fyne/v2"

// logBindError logs Fyne data-binding failures (normally rare).
func logBindError(context string, err error) {
	if err != nil {
		fyne.LogError(context, err)
	}
}
