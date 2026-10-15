// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"testing"
)

func TestVerbosityLevel_Validate(t *testing.T) {
	tests := []struct {
		name    string
		level   VerbosityLevel
		wantErr bool
	}{
		{
			name:    "empty string (normal) is valid",
			level:   VerbosityNormal,
			wantErr: false,
		},
		{
			name:    "verbose is valid",
			level:   VerbosityVerbose,
			wantErr: false,
		},
		{
			name:    "extra is valid",
			level:   VerbosityExtra,
			wantErr: false,
		},
		{
			name:    "max is valid",
			level:   VerbosityMax,
			wantErr: false,
		},
		{
			name:    "invalid value",
			level:   VerbosityLevel("invalid"),
			wantErr: true,
		},
		{
			name:    "v is invalid (should use verbose)",
			level:   VerbosityLevel("v"),
			wantErr: true,
		},
		{
			name:    "vvv is invalid (should use max)",
			level:   VerbosityLevel("vvv"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.level.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("VerbosityLevel.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerbosityLevel_Constants(t *testing.T) {
	tests := []struct {
		name     string
		level    VerbosityLevel
		expected string
	}{
		{
			name:     "normal is empty string",
			level:    VerbosityNormal,
			expected: "",
		},
		{
			name:     "verbose equals 'verbose'",
			level:    VerbosityVerbose,
			expected: "verbose",
		},
		{
			name:     "extra equals 'extra'",
			level:    VerbosityExtra,
			expected: "extra",
		},
		{
			name:     "max equals 'max'",
			level:    VerbosityMax,
			expected: "max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.level) != tt.expected {
				t.Errorf("VerbosityLevel constant = %q, want %q", tt.level, tt.expected)
			}
		})
	}
}

func TestTimestampFormat_Validate(t *testing.T) {
	tests := []struct {
		name    string
		format  TimestampFormat
		wantErr bool
	}{
		{
			name:    "empty string (default) is valid",
			format:  TimestampDefault,
			wantErr: false,
		},
		{
			name:    "none is valid",
			format:  TimestampNone,
			wantErr: false,
		},
		{
			name:    "unformatted is valid",
			format:  TimestampUnformatted,
			wantErr: false,
		},
		{
			name:    "delta is valid",
			format:  TimestampDelta,
			wantErr: false,
		},
		{
			name:    "date is valid",
			format:  TimestampDate,
			wantErr: false,
		},
		{
			name:    "delta-since-first is valid",
			format:  TimestampDeltaSinceFirst,
			wantErr: false,
		},
		{
			name:    "invalid value",
			format:  TimestampFormat("invalid"),
			wantErr: true,
		},
		{
			name:    "epoch is invalid (should use unformatted)",
			format:  TimestampFormat("epoch"),
			wantErr: true,
		},
		{
			name:    "default as string is invalid (should be empty)",
			format:  TimestampFormat("default"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.format.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("TimestampFormat.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimestampFormat_Constants(t *testing.T) {
	tests := []struct {
		name     string
		format   TimestampFormat
		expected string
	}{
		{
			name:     "default is empty string",
			format:   TimestampDefault,
			expected: "",
		},
		{
			name:     "none equals 'none'",
			format:   TimestampNone,
			expected: "none",
		},
		{
			name:     "unformatted equals 'unformatted'",
			format:   TimestampUnformatted,
			expected: "unformatted",
		},
		{
			name:     "delta equals 'delta'",
			format:   TimestampDelta,
			expected: "delta",
		},
		{
			name:     "date equals 'date'",
			format:   TimestampDate,
			expected: "date",
		},
		{
			name:     "delta-since-first equals 'delta-since-first'",
			format:   TimestampDeltaSinceFirst,
			expected: "delta-since-first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.format) != tt.expected {
				t.Errorf("TimestampFormat constant = %q, want %q", tt.format, tt.expected)
			}
		})
	}
}

func TestPrintDataFormat_Validate(t *testing.T) {
	tests := []struct {
		name    string
		format  PrintDataFormat
		wantErr bool
	}{
		{
			name:    "empty string (none) is valid",
			format:  PrintDataNone,
			wantErr: false,
		},
		{
			name:    "hex is valid",
			format:  PrintDataHex,
			wantErr: false,
		},
		{
			name:    "hex-with-link is valid",
			format:  PrintDataHexWithLink,
			wantErr: false,
		},
		{
			name:    "ascii is valid",
			format:  PrintDataASCII,
			wantErr: false,
		},
		{
			name:    "ascii-with-link is valid",
			format:  PrintDataASCIIWithLink,
			wantErr: false,
		},
		{
			name:    "invalid value",
			format:  PrintDataFormat("invalid"),
			wantErr: true,
		},
		{
			name:    "X is invalid (should use hex)",
			format:  PrintDataFormat("X"),
			wantErr: true,
		},
		{
			name:    "none as string is invalid (should be empty)",
			format:  PrintDataFormat("none"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.format.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PrintDataFormat.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrintDataFormat_Constants(t *testing.T) {
	tests := []struct {
		name     string
		format   PrintDataFormat
		expected string
	}{
		{
			name:     "none is empty string",
			format:   PrintDataNone,
			expected: "",
		},
		{
			name:     "hex equals 'hex'",
			format:   PrintDataHex,
			expected: "hex",
		},
		{
			name:     "hex-with-link equals 'hex-with-link'",
			format:   PrintDataHexWithLink,
			expected: "hex-with-link",
		},
		{
			name:     "ascii equals 'ascii'",
			format:   PrintDataASCII,
			expected: "ascii",
		},
		{
			name:     "ascii-with-link equals 'ascii-with-link'",
			format:   PrintDataASCIIWithLink,
			expected: "ascii-with-link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.format) != tt.expected {
				t.Errorf("PrintDataFormat constant = %q, want %q", tt.format, tt.expected)
			}
		})
	}
}

// TestEnumMapping_Verbosity tests that the enum values correctly map to their intended usage
func TestEnumMapping_Verbosity(t *testing.T) {
	tests := []struct {
		name         string
		cliFlag      string
		expectedEnum VerbosityLevel
		tcpdumpFlag  string // what tcpdump flag this should produce
	}{
		{
			name:         "no flag means normal (no tcpdump verbosity)",
			cliFlag:      "",
			expectedEnum: VerbosityNormal,
			tcpdumpFlag:  "(none)",
		},
		{
			name:         "verbose produces -v",
			cliFlag:      "verbose",
			expectedEnum: VerbosityVerbose,
			tcpdumpFlag:  "-v",
		},
		{
			name:         "extra produces -vv",
			cliFlag:      "extra",
			expectedEnum: VerbosityExtra,
			tcpdumpFlag:  "-vv",
		},
		{
			name:         "max produces -vvv",
			cliFlag:      "max",
			expectedEnum: VerbosityMax,
			tcpdumpFlag:  "-vvv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := VerbosityLevel(tt.cliFlag)
			if level != tt.expectedEnum {
				t.Errorf("VerbosityLevel(%q) = %q, want %q", tt.cliFlag, level, tt.expectedEnum)
			}
			if err := level.Validate(); err != nil {
				t.Errorf("VerbosityLevel(%q).Validate() error = %v", tt.cliFlag, err)
			}
			t.Logf("✓ --verbosity=%s → %s → tcpdump %s", tt.cliFlag, tt.expectedEnum, tt.tcpdumpFlag)
		})
	}
}

// TestEnumMapping_Timestamp tests that the enum values correctly map to their intended usage
func TestEnumMapping_Timestamp(t *testing.T) {
	tests := []struct {
		name         string
		cliFlag      string
		expectedEnum TimestampFormat
		tcpdumpFlag  string
	}{
		{
			name:         "default means normal timestamps",
			cliFlag:      "",
			expectedEnum: TimestampDefault,
			tcpdumpFlag:  "(default)",
		},
		{
			name:         "none produces -t",
			cliFlag:      "none",
			expectedEnum: TimestampNone,
			tcpdumpFlag:  "-t",
		},
		{
			name:         "unformatted produces -tt",
			cliFlag:      "unformatted",
			expectedEnum: TimestampUnformatted,
			tcpdumpFlag:  "-tt",
		},
		{
			name:         "delta produces -ttt",
			cliFlag:      "delta",
			expectedEnum: TimestampDelta,
			tcpdumpFlag:  "-ttt",
		},
		{
			name:         "date produces -tttt",
			cliFlag:      "date",
			expectedEnum: TimestampDate,
			tcpdumpFlag:  "-tttt",
		},
		{
			name:         "delta-since-first produces -ttttt",
			cliFlag:      "delta-since-first",
			expectedEnum: TimestampDeltaSinceFirst,
			tcpdumpFlag:  "-ttttt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := TimestampFormat(tt.cliFlag)
			if format != tt.expectedEnum {
				t.Errorf("TimestampFormat(%q) = %q, want %q", tt.cliFlag, format, tt.expectedEnum)
			}
			if err := format.Validate(); err != nil {
				t.Errorf("TimestampFormat(%q).Validate() error = %v", tt.cliFlag, err)
			}
			t.Logf("✓ --timestamp-format=%s → %s → tcpdump %s", tt.cliFlag, tt.expectedEnum, tt.tcpdumpFlag)
		})
	}
}

// TestEnumMapping_PrintData tests that the enum values correctly map to their intended usage
func TestEnumMapping_PrintData(t *testing.T) {
	tests := []struct {
		name         string
		cliFlag      string
		expectedEnum PrintDataFormat
		tcpdumpFlag  string
	}{
		{
			name:         "default means no data printing",
			cliFlag:      "",
			expectedEnum: PrintDataNone,
			tcpdumpFlag:  "(none)",
		},
		{
			name:         "hex produces -x",
			cliFlag:      "hex",
			expectedEnum: PrintDataHex,
			tcpdumpFlag:  "-x",
		},
		{
			name:         "hex-with-link produces -xx",
			cliFlag:      "hex-with-link",
			expectedEnum: PrintDataHexWithLink,
			tcpdumpFlag:  "-xx",
		},
		{
			name:         "ascii produces -A",
			cliFlag:      "ascii",
			expectedEnum: PrintDataASCII,
			tcpdumpFlag:  "-A",
		},
		{
			name:         "ascii-with-link produces -AA",
			cliFlag:      "ascii-with-link",
			expectedEnum: PrintDataASCIIWithLink,
			tcpdumpFlag:  "-AA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := PrintDataFormat(tt.cliFlag)
			if format != tt.expectedEnum {
				t.Errorf("PrintDataFormat(%q) = %q, want %q", tt.cliFlag, format, tt.expectedEnum)
			}
			if err := format.Validate(); err != nil {
				t.Errorf("PrintDataFormat(%q).Validate() error = %v", tt.cliFlag, err)
			}
			t.Logf("✓ --print-data=%s → %s → tcpdump %s", tt.cliFlag, tt.expectedEnum, tt.tcpdumpFlag)
		})
	}
}
