package core

import "context"

// Reporter receives progress and log lines from the core library (CLI or GUI implement this).
type Reporter interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	// Progress reports byte-level progress for a single long-running step (e.g. one file).
	Progress(readBytes, writtenBytes, totalBytes int64, step string)
}

// NopReporter discards output.
type NopReporter struct{}

func (NopReporter) Info(string)                      {}
func (NopReporter) Warn(string)                      {}
func (NopReporter) Error(string)                     {}
func (NopReporter) Progress(_, _, _ int64, _ string) {}

// ContextReporter wraps another reporter and stops calling it when ctx is done.
type ContextReporter struct {
	Ctx context.Context
	R   Reporter
}

func (c ContextReporter) Info(msg string) {
	if c.Ctx.Err() != nil {
		return
	}
	c.R.Info(msg)
}

func (c ContextReporter) Warn(msg string) {
	if c.Ctx.Err() != nil {
		return
	}
	c.R.Warn(msg)
}

func (c ContextReporter) Error(msg string) {
	if c.Ctx.Err() != nil {
		return
	}
	c.R.Error(msg)
}

func (c ContextReporter) Progress(a, b, t int64, s string) {
	if c.Ctx.Err() != nil {
		return
	}
	c.R.Progress(a, b, t, s)
}
