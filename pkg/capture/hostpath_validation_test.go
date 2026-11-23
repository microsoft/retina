// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"errors"
	"path"
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
		{name: "embedded double quote", raw: `pwn"whoami`, baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "embedded angle bracket", raw: "job<name", baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "embedded pipe", raw: "job|name", baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "embedded wildcard", raw: "job*name", baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "embedded colon", raw: "job:name", baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "embedded ampersand", raw: "pwn&whoami", baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "embedded dollar and parens", raw: "pwn$(id)", baseDir: base, wantErr: ErrHostPathInvalidChars},

		// acceptance cases
		{name: "bare name", raw: "retina", baseDir: base, want: base + "/retina"},
		{name: "nested subpath", raw: "job1/out", baseDir: base, want: base + "/job1/out"},
		{name: "trailing slash cleaned", raw: "job/", baseDir: base, want: base + "/job"},
		{name: "redundant separators cleaned", raw: "job//./out", baseDir: base, want: base + "/job/out"},
		{name: "root base dir accepts subpath", raw: "captures", baseDir: "/", want: "/captures"},
		{name: "root base dir accepts nested subpath", raw: "a/b", baseDir: "/", want: "/a/b"},
		{name: "embedded space is valid", raw: "job name", baseDir: base, want: base + "/job name"},
		{name: "embedded at sign is valid", raw: "job@name", baseDir: base, want: base + "/job@name"},
		{name: "embedded plus is valid", raw: "job+name", baseDir: base, want: base + "/job+name"},
		{name: "embedded dollar sign is valid", raw: "job$name", baseDir: base, want: base + "/job$name"},
		{name: "unicode is valid", raw: "captüre", baseDir: base, want: base + "/captüre"},

		// defaulting
		{name: "default base used when empty", raw: "x", baseDir: "", want: DefaultHostPathBaseDir + "/x"},

		// invalid base
		{name: "relative base rejected", raw: "x", baseDir: "captures", wantErr: ErrHostPathBaseDir},
		{name: "base dir with unsafe chars rejected", raw: "x", baseDir: base + "&evil", wantErr: ErrHostPathInvalidChars},
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

func TestValidateResolvedHostPath(t *testing.T) {
	const (
		base       = "/var/log/retina/captures"
		customBase = "/mnt/captures"
	)

	tests := []struct {
		name     string
		p        string
		baseDir  string
		wantBase string
		wantRel  string
		wantErr  error
	}{
		// rejection cases
		{name: "empty", p: "", baseDir: base, wantErr: ErrHostPathEmpty},
		{name: "root", p: "/", baseDir: base, wantErr: ErrHostPathEscapesBase},
		{name: "other absolute path", p: "/etc/kubernetes", baseDir: base, wantErr: ErrHostPathEscapesBase},
		{name: "parent of base", p: "/var/log/retina", baseDir: base, wantErr: ErrHostPathEscapesBase},
		{name: "sibling sharing the base prefix", p: base + "-evil", baseDir: base, wantErr: ErrHostPathEscapesBase},
		{name: "traversal out of base", p: base + "/../../../etc", baseDir: base, wantErr: ErrHostPathTraversal},
		{name: "traversal within base", p: base + "/a/../b", baseDir: base, wantErr: ErrHostPathTraversal},
		{name: "traversal with backslashes", p: base + `\..\..\..\etc`, baseDir: base, wantErr: ErrHostPathTraversal},
		{name: "relative", p: "retina", baseDir: base, wantErr: ErrResolvedHostPathRelative},
		{name: "relative traversal", p: "../../etc", baseDir: base, wantErr: ErrResolvedHostPathRelative},
		{name: "windows leading backslash", p: `\var\log\retina\captures\x`, baseDir: base, wantErr: ErrResolvedHostPathRelative},
		{name: "windows drive letter", p: `C:\Windows`, baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "backslash inside a segment on linux", p: `/var/log/retina\captures/x`, baseDir: base, wantErr: ErrHostPathEscapesBase},
		{name: "backslash at the base boundary on linux", p: base + `\retina`, baseDir: base, wantErr: ErrHostPathEscapesBase},
		{name: "unsafe characters", p: base + "/pwn&whoami", baseDir: base, wantErr: ErrHostPathInvalidChars},
		{name: "outside a custom base", p: base + "/x", baseDir: customBase, wantErr: ErrHostPathEscapesBase},
		{name: "invalid base", p: base + "/x", baseDir: "captures", wantErr: ErrHostPathBaseDir},

		// acceptance cases
		{name: "subpath", p: base + "/retina", baseDir: base, wantBase: base, wantRel: "retina"},
		{name: "nested subpath", p: base + "/a/b", baseDir: base, wantBase: base, wantRel: "a/b"},
		{name: "base itself", p: base, baseDir: base, wantBase: base, wantRel: ""},
		{name: "cleaned", p: base + "//retina/./", baseDir: base + "/", wantBase: base, wantRel: "retina"},
		{name: "backslash inside a subpath", p: base + `/win\sub`, baseDir: base, wantBase: base, wantRel: `win\sub`},
		{name: "custom base", p: customBase + "/retina", baseDir: customBase, wantBase: customBase, wantRel: "retina"},
		{name: "root base", p: "/a/b", baseDir: "/", wantBase: "/", wantRel: "a/b"},
		{name: "default base when empty", p: DefaultHostPathBaseDir + "/retina", baseDir: "", wantBase: DefaultHostPathBaseDir, wantRel: "retina"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotRel, err := ValidateResolvedHostPath(tt.p, tt.baseDir)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				if gotBase != "" || gotRel != "" {
					t.Fatalf("expected empty results on error, got %q, %q", gotBase, gotRel)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotBase != tt.wantBase || gotRel != tt.wantRel {
				t.Fatalf("got (%q, %q), want (%q, %q)", gotBase, gotRel, tt.wantBase, tt.wantRel)
			}
		})
	}
}

// Every path the capture translator resolves must also pass the download-time check with the same base directory,
// otherwise valid captures could not be downloaded.
func TestValidateResolvedHostPath_AcceptsResolvedHostPaths(t *testing.T) {
	for _, base := range []string{DefaultHostPathBaseDir, "/mnt/captures/", "/"} {
		for _, sub := range []string{"retina", "a/b", "job name", `win\sub`} {
			resolved, err := validateHostPath(sub, base)
			if err != nil {
				t.Fatalf("validateHostPath(%q, %q): %v", sub, base, err)
			}
			gotBase, gotRel, err := ValidateResolvedHostPath(resolved, base)
			if err != nil {
				t.Fatalf("ValidateResolvedHostPath(%q, %q): %v", resolved, base, err)
			}
			if got := path.Join(gotBase, gotRel); got != resolved || gotRel != sub {
				t.Fatalf("got (%q, %q), want %q relative %q", gotBase, gotRel, resolved, sub)
			}
		}
	}
}
