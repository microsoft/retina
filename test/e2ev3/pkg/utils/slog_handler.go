// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/term"
)

// StepHandler is an slog.Handler that produces structured log lines with
// workflow/test/step context rendered as a bracketed prefix.
//
// Output format:
//
//	15:04:05 INFO [workflow/test/step] message key=value ...
//
// The "workflow", "test", and "step" attributes are absorbed into the prefix
// and not printed as key=value pairs. When no prefix parts are set the
// brackets are omitted entirely.
type StepHandler struct {
	w        io.Writer
	level    slog.Level
	workflow string
	test     string
	step     string
	prefix   string
	color    bool
	attrs    []slog.Attr
	mu       *sync.Mutex
}

func NewStepHandler(w io.Writer, level slog.Level) *StepHandler {
	c := false
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		c = isTerminal(f.Fd())
	}
	return &StepHandler{w: w, level: level, color: c, mu: &sync.Mutex{}}
}

// NewStepHandlerWithColor creates a handler with explicit color control (for tests).
func NewStepHandlerWithColor(w io.Writer, level slog.Level, color bool) *StepHandler {
	return &StepHandler{w: w, level: level, color: color, mu: &sync.Mutex{}}
}

func (h *StepHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *StepHandler) Handle(ctx context.Context, r slog.Record) error {
	var buf bytes.Buffer

	// Start with any prefix from context (set by StepLogger),
	// then check handler-level prefix (from WithAttrs).
	prefix := Prefix(ctx)
	if prefix == "" {
		prefix = h.prefix
	}

	// Also check handler-level and record-level attrs for prefix/workflow/test/step.
	// "prefix" overrides everything; legacy workflow/test/step build a prefix if no "prefix" attr.
	workflow, test, step := h.workflow, h.test, h.step

	var extra []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "prefix":
			prefix = a.Value.String()
		case "workflow":
			workflow = a.Value.String()
		case "test":
			test = a.Value.String()
		case "step":
			step = a.Value.String()
		default:
			extra = append(extra, a)
		}
		return true
	})

	// If no explicit prefix, build from workflow/test/step parts.
	if prefix == "" {
		prefix = buildPrefix(workflow, test, step)
	}

	// If still empty, try caller detection from stack.
	if prefix == "" {
		cw, _, cs := callerPrefix()
		prefix = buildPrefix(cw, "", cs)
	}

	// Timestamp and level always come first.
	fmt.Fprintf(&buf, "%s %s ",
		r.Time.Format("15:04:05"),
		r.Level.String())

	// Render the [prefix] bracket.
	if prefix != "" {
		if h.color {
			code := colorForPrefix(prefix)
			fmt.Fprintf(&buf, "\033[%dm[%s]\033[0m ", code, prefix)
		} else {
			fmt.Fprintf(&buf, "[%s] ", prefix)
		}
	}

	buf.WriteString(r.Message)

	// Pre-attached attrs (from WithAttrs), skipping prefix keys.
	for _, a := range h.attrs {
		fmt.Fprintf(&buf, " %s=%s", a.Key, a.Value)
	}
	// Record-level attrs (prefix keys already absorbed above).
	for _, a := range extra {
		fmt.Fprintf(&buf, " %s=%s", a.Key, a.Value)
	}

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *StepHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	workflow, test, step, prefix := h.workflow, h.test, h.step, h.prefix
	var remaining []slog.Attr
	for _, a := range attrs {
		switch a.Key {
		case "prefix":
			prefix = a.Value.String()
		case "workflow":
			workflow = a.Value.String()
		case "test":
			test = a.Value.String()
		case "step":
			step = a.Value.String()
		default:
			remaining = append(remaining, a)
		}
	}
	return &StepHandler{
		w:        h.w,
		level:    h.level,
		workflow: workflow,
		test:     test,
		step:     step,
		prefix:   prefix,
		color:    h.color,
		attrs:    append(slices.Clone(h.attrs), remaining...),
		mu:       h.mu,
	}
}

func (h *StepHandler) WithGroup(name string) slog.Handler {
	return h
}

// buildPrefix joins non-empty parts with "/".
func buildPrefix(parts ...string) string {
	var buf bytes.Buffer
	for _, p := range parts {
		if p == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('/')
		}
		buf.WriteString(p)
	}
	return buf.String()
}

// colorForPrefix returns a deterministic ANSI color code for the given prefix string.
func colorForPrefix(prefix string) int {
	codes := []int{31, 32, 33, 34, 35, 36, 91, 92, 93, 94, 95, 96}
	h := fnv32a(prefix)
	return codes[h%uint32(len(codes))]
}

func fnv32a(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// isTerminal checks if the given file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

const e2ev3Prefix = "retina/test/e2ev3/"

// callerPrefix scans the call stack for e2ev3 types and returns
// (workflow, test, step). It identifies the outermost Workflow receiver
// as the workflow name and the innermost non-Workflow receiver as the step.
func callerPrefix() (workflow, test, step string) {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.Function, e2ev3Prefix) {
			if !more {
				break
			}
			continue
		}
		typeName, pkgName := extractCallerInfo(frame.Function)
		if typeName == "" {
			if !more {
				break
			}
			continue
		}
		kebab := toKebabCase(typeName)
		if kebab == "workflow" || kebab == "step" {
			// Generic type — use package name as the workflow identifier.
			workflow = toKebabCase(pkgName)
		} else if kebab == "slog-writer" {
			// io.Writer adapter — not a real step, skip it.
			if !more {
				break
			}
			continue
		} else if step == "" {
			step = kebab
		}
		if !more {
			break
		}
	}
	return workflow, test, step
}

// extractCallerInfo extracts the type name and package name from a fully
// qualified function name like "github.com/.../pkg/kubernetes.(*PortForward).Do".
func extractCallerInfo(funcName string) (typeName, pkgName string) {
	// Get last path component: "kubernetes.(*PortForward).Do"
	base := path.Base(funcName)
	// Split on ".": ["kubernetes", "(*PortForward)", "Do"]
	parts := strings.SplitN(base, ".", 3)
	if len(parts) < 2 {
		return "", ""
	}
	pkgName = parts[0]
	receiver := parts[1]
	// Strip pointer/paren: "(*PortForward)" → "PortForward"
	receiver = strings.TrimPrefix(receiver, "(*")
	receiver = strings.TrimSuffix(receiver, ")")
	receiver = strings.TrimPrefix(receiver, "*")
	return receiver, pkgName
}

// toKebabCase converts PascalCase to kebab-case, keeping consecutive
// uppercase letters together (e.g. "InstallNPM" → "install-npm").
func toKebabCase(s string) string {
	var buf bytes.Buffer
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) {
					buf.WriteByte('-')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					buf.WriteByte('-')
				}
			}
			buf.WriteRune(unicode.ToLower(r))
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
