// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
)

// StepHandler is an slog.Handler that writes the "step" attribute as a prefix
// before the standard time/level/msg fields.
//
// Output format:
//
//	[step-name] 15:04:05 INFO message key=value ...
//
// When no step is set, the prefix is omitted.
type StepHandler struct {
	w     io.Writer
	level slog.Level
	step  string
	attrs []slog.Attr
	mu    *sync.Mutex
}

func NewStepHandler(w io.Writer, level slog.Level) *StepHandler {
	return &StepHandler{w: w, level: level, mu: &sync.Mutex{}}
}

func (h *StepHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *StepHandler) Handle(ctx context.Context, r slog.Record) error {
	var buf bytes.Buffer

	workflow := WorkflowName(ctx)
	switch {
	case workflow != "" && h.step != "":
		fmt.Fprintf(&buf, "[%s/%s] ", workflow, h.step)
	case workflow != "":
		fmt.Fprintf(&buf, "[%s] ", workflow)
	case h.step != "":
		fmt.Fprintf(&buf, "[%s] ", h.step)
	}

	fmt.Fprintf(&buf, "%s %s %s",
		r.Time.Format("15:04:05"),
		r.Level.String(),
		r.Message)

	for _, a := range h.attrs {
		fmt.Fprintf(&buf, " %s=%s", a.Key, a.Value)
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&buf, " %s=%s", a.Key, a.Value)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *StepHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	step := h.step
	var remaining []slog.Attr
	for _, a := range attrs {
		if a.Key == "step" {
			step = a.Value.String()
		} else {
			remaining = append(remaining, a)
		}
	}
	return &StepHandler{
		w:     h.w,
		level: h.level,
		step:  step,
		attrs: append(slices.Clone(h.attrs), remaining...),
		mu:    h.mu,
	}
}

func (h *StepHandler) WithGroup(name string) slog.Handler {
	return h
}
