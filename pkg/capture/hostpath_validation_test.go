// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateHostPath(t *testing.T) {
	const base = "/var/log/retina/captures"

	tests := []struct {
		name    string
		raw     string
		baseDir string
		want    string
		wantErr error
	}{
		// rejection cases
		{name: "empty", raw: "", baseDir: base, wantErr: ErrHostPathEmpty},
		{name: "dot only resolves to empty", raw: ".", baseDir: base, wantErr: ErrHostPathEmpty},
		{name: "absolute posix", raw: "/tmp/retina", baseDir: base, wantErr: ErrHostPathAbsolute},
		{name: "absolute posix with traversal", raw: "/var/log/../etc", baseDir: base, wantErr: ErrHostPathAbsolute},
		{name: "absolute windows backslash", raw: `\tmp\retina`, baseDir: base, wantErr: ErrHostPathAbsolute},
		{name: "absolute windows drive letter", raw: `C:\evil`, baseDir: base, wantErr: ErrHostPathAbsolute},
		{name: "absolute windows drive letter forward", raw: "c:/evil", baseDir: base, wantErr: ErrHostPathAbsolute},
		{name: "traversal raw", raw: "../etc", baseDir: base, wantErr: ErrHostPathTraversal},
		{name: "traversal mid", raw: "foo/../bar", baseDir: base, wantErr: ErrHostPathTraversal},
		{name: "traversal backslash", raw: `foo\..\bar`, baseDir: base, wantErr: ErrHostPathTraversal},
		{name: "traversal escaping base", raw: "../../etc", baseDir: base, wantErr: ErrHostPathTraversal},

		// acceptance cases
		{name: "bare name", raw: "retina", baseDir: base, want: base + "/retina"},
		{name: "nested subpath", raw: "job1/out", baseDir: base, want: base + "/job1/out"},
		{name: "trailing slash cleaned", raw: "job/", baseDir: base, want: base + "/job"},
		{name: "redundant separators cleaned", raw: "job//./out", baseDir: base, want: base + "/job/out"},

		// defaulting
		{name: "default base used when empty", raw: "x", baseDir: "", want: DefaultHostPathBaseDir + "/x"},

		// invalid base
		{name: "relative base rejected", raw: "x", baseDir: "captures", wantErr: ErrHostPathBaseDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateHostPath(tt.raw, tt.baseDir)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				if got != "" {
					t.Fatalf("expected empty result on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestValidateHostPath_ResultsAlwaysUnderBase is a property-style assertion: for
// every accepted input across a small enumerated set, the cleaned result must be
// the base or be nested under base + separator. This guards against future
// regressions where validation might let through an input whose joined form
// escapes the base directory.
func TestValidateHostPath_ResultsAlwaysUnderBase(t *testing.T) {
	const base = "/var/log/retina/captures"
	inputs := []string{
		"a", "a/b", "a/b/c", "x.pcap", "deep/nested/path/name",
		"with-hyphen", "with_underscore", "with.dots",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			got, err := validateHostPath(in, base)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", in, err)
			}
			if got != base && !strings.HasPrefix(got, base+"/") {
				t.Fatalf("%q resolved to %q which is not under %q", in, got, base)
			}
		})
	}
}
