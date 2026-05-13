// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"errors"
	"testing"
)

func TestValidateHostPath(t *testing.T) {
	allow := []string{"/var/log/retina/captures", "/mnt/captures"}

	tests := []struct {
		name     string
		raw      string
		prefixes []string
		want     string
		wantErr  error
	}{
		{
			name:    "empty",
			raw:     "",
			wantErr: ErrHostPathEmpty,
		},
		{
			name:     "relative path rejected",
			raw:      "captures/out",
			prefixes: allow,
			wantErr:  ErrHostPathNotAbsolute,
		},
		{
			name:     "traversal literal rejected",
			raw:      "/var/log/retina/captures/../../etc",
			prefixes: allow,
			wantErr:  ErrHostPathTraversal,
		},
		{
			name:     "exact prefix accepted",
			raw:      "/var/log/retina/captures",
			prefixes: allow,
			want:     "/var/log/retina/captures",
		},
		{
			name:     "nested path accepted",
			raw:      "/var/log/retina/captures/job-1",
			prefixes: allow,
			want:     "/var/log/retina/captures/job-1",
		},
		{
			name:     "trailing slash cleaned",
			raw:      "/var/log/retina/captures/",
			prefixes: allow,
			want:     "/var/log/retina/captures",
		},
		{
			name:     "redundant separators cleaned",
			raw:      "/var/log/retina/captures//job/./out",
			prefixes: allow,
			want:     "/var/log/retina/captures/job/out",
		},
		{
			name:     "prefix confusion rejected",
			raw:      "/var/log/retina/captures-evil/x",
			prefixes: allow,
			wantErr:  ErrHostPathNotAllowed,
		},
		{
			name:     "other allowlisted prefix accepted",
			raw:      "/mnt/captures/foo",
			prefixes: allow,
			want:     "/mnt/captures/foo",
		},
		{
			name:     "outside allowlist rejected",
			raw:      "/etc/shadow",
			prefixes: allow,
			wantErr:  ErrHostPathNotAllowed,
		},
		{
			name: "default prefix used when none configured",
			raw:  "/var/log/retina/captures/x",
			want: "/var/log/retina/captures/x",
		},
		{
			name:    "default prefix rejects others",
			raw:     "/tmp/anything",
			wantErr: ErrHostPathNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateHostPath(tt.raw, tt.prefixes)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
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
