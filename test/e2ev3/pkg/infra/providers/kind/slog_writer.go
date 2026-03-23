// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"bytes"
	"context"
	"log/slog"
)

// slogWriter is an io.Writer that logs each complete line through slog at the given level.
// Partial lines are buffered until a newline is received.
type slogWriter struct {
	level  slog.Level
	source string
	buf    []byte
}

func (w *slogWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(bytes.TrimRight(w.buf[:idx], "\r"))
		w.buf = w.buf[idx+1:]
		if line != "" {
			slog.Log(context.Background(), w.level, line, "source", w.source)
		}
	}
	return len(p), nil
}

// Flush logs any remaining buffered content not terminated by a newline.
func (w *slogWriter) Flush() {
	if len(w.buf) > 0 {
		line := string(bytes.TrimRight(w.buf, "\r\n"))
		if line != "" {
			slog.Log(context.Background(), w.level, line, "source", w.source)
		}
		w.buf = nil
	}
}
