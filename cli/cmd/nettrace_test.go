// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"net"
	"testing"
)

func TestValidateFilterIP(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIP  net.IP
		wantErr bool
	}{
		// Valid cases
		{
			name:    "valid IPv4",
			input:   "10.0.0.1",
			wantIP:  net.ParseIP("10.0.0.1"),
			wantErr: false,
		},
		{
			name:    "valid IPv4 192.168",
			input:   "192.168.1.1",
			wantIP:  net.ParseIP("192.168.1.1"),
			wantErr: false,
		},
		{
			name:    "valid IPv6 loopback",
			input:   "::1",
			wantIP:  net.ParseIP("::1"),
			wantErr: false,
		},
		{
			name:    "valid IPv6 full",
			input:   "2001:db8::1",
			wantIP:  net.ParseIP("2001:db8::1"),
			wantErr: false,
		},
		{
			name:    "empty string - no filter",
			input:   "",
			wantIP:  nil,
			wantErr: false,
		},
		// Security: Injection attempts
		{
			name:    "injection attempt - semicolon command",
			input:   "10.0.0.1; rm -rf /",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "injection attempt - backtick command",
			input:   "10.0.0.1`whoami`",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "injection attempt - dollar command",
			input:   "10.0.0.1$(whoami)",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "injection attempt - pipe",
			input:   "10.0.0.1 | cat /etc/passwd",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "injection attempt - newline",
			input:   "10.0.0.1\nrm -rf /",
			wantIP:  nil,
			wantErr: true,
		},
		// Invalid IPs
		{
			name:    "not an IP - text",
			input:   "not-an-ip",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "invalid octet - 256",
			input:   "10.0.0.256",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "invalid format - too many octets",
			input:   "10.0.0.1.5",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "invalid - CIDR notation",
			input:   "10.0.0.0/24",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "invalid - hostname",
			input:   "example.com",
			wantIP:  nil,
			wantErr: true,
		},
		{
			name:    "invalid - negative number",
			input:   "-1.0.0.1",
			wantIP:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIP, err := ValidateFilterIP(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilterIP(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			// Compare IPs
			if tt.wantIP == nil && gotIP == nil {
				return // Both nil, OK
			}
			if tt.wantIP == nil || gotIP == nil {
				t.Errorf("ValidateFilterIP(%q) = %v, want %v", tt.input, gotIP, tt.wantIP)
				return
			}
			if !tt.wantIP.Equal(gotIP) {
				t.Errorf("ValidateFilterIP(%q) = %v, want %v", tt.input, gotIP, tt.wantIP)
			}
		})
	}
}

func TestValidateFilterCIDR(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCIDR string // Expected CIDR string representation
		wantErr  bool
	}{
		// Valid cases
		{
			name:     "valid /24",
			input:    "10.0.0.0/24",
			wantCIDR: "10.0.0.0/24",
			wantErr:  false,
		},
		{
			name:     "valid /16",
			input:    "192.168.0.0/16",
			wantCIDR: "192.168.0.0/16",
			wantErr:  false,
		},
		{
			name:     "valid /8",
			input:    "10.0.0.0/8",
			wantCIDR: "10.0.0.0/8",
			wantErr:  false,
		},
		{
			name:     "valid /32 single host",
			input:    "10.0.0.1/32",
			wantCIDR: "10.0.0.1/32",
			wantErr:  false,
		},
		{
			name:     "valid - normalizes to network address",
			input:    "10.0.0.5/24",
			wantCIDR: "10.0.0.0/24", // Normalized
			wantErr:  false,
		},
		{
			name:     "empty string - no filter",
			input:    "",
			wantCIDR: "",
			wantErr:  false,
		},
		// Security: Injection attempts
		{
			name:     "injection attempt - semicolon",
			input:    "10.0.0.0/24; rm -rf /",
			wantCIDR: "",
			wantErr:  true,
		},
		{
			name:     "injection attempt - backtick",
			input:    "10.0.0.0/24`whoami`",
			wantCIDR: "",
			wantErr:  true,
		},
		{
			name:     "injection attempt - newline",
			input:    "10.0.0.0/24\ncat /etc/passwd",
			wantCIDR: "",
			wantErr:  true,
		},
		// Invalid CIDRs
		{
			name:     "invalid - no mask",
			input:    "10.0.0.0",
			wantCIDR: "",
			wantErr:  true,
		},
		{
			name:     "invalid - mask too large",
			input:    "10.0.0.0/33",
			wantCIDR: "",
			wantErr:  true,
		},
		{
			name:     "invalid - negative mask",
			input:    "10.0.0.0/-1",
			wantCIDR: "",
			wantErr:  true,
		},
		{
			name:     "invalid - text",
			input:    "not-a-cidr",
			wantCIDR: "",
			wantErr:  true,
		},
		{
			name:     "invalid - hostname",
			input:    "example.com/24",
			wantCIDR: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCIDR, err := ValidateFilterCIDR(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilterCIDR(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			// Compare CIDRs
			if tt.wantCIDR == "" && gotCIDR == nil {
				return // Both empty, OK
			}
			if tt.wantCIDR == "" || gotCIDR == nil {
				t.Errorf("ValidateFilterCIDR(%q) = %v, want %v", tt.input, gotCIDR, tt.wantCIDR)
				return
			}
			if gotCIDR.String() != tt.wantCIDR {
				t.Errorf("ValidateFilterCIDR(%q) = %v, want %v", tt.input, gotCIDR.String(), tt.wantCIDR)
			}
		})
	}
}

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFormat TraceOutputFormat
		wantErr    bool
	}{
		// Valid cases
		{
			name:       "table",
			input:      "table",
			wantFormat: TraceOutputTable,
			wantErr:    false,
		},
		{
			name:       "json",
			input:      "json",
			wantFormat: TraceOutputJSON,
			wantErr:    false,
		},
		{
			name:       "empty defaults to table",
			input:      "",
			wantFormat: TraceOutputTable,
			wantErr:    false,
		},
		// Invalid cases
		{
			name:       "invalid - yaml",
			input:      "yaml",
			wantFormat: "",
			wantErr:    true,
		},
		{
			name:       "invalid - xml",
			input:      "xml",
			wantFormat: "",
			wantErr:    true,
		},
		{
			name:       "invalid - random",
			input:      "notaformat",
			wantFormat: "",
			wantErr:    true,
		},
		{
			name:       "invalid - TABLE uppercase",
			input:      "TABLE",
			wantFormat: "",
			wantErr:    true,
		},
		{
			name:       "invalid - JSON uppercase",
			input:      "JSON",
			wantFormat: "",
			wantErr:    true,
		},
		// Security: injection attempts
		{
			name:       "injection - semicolon",
			input:      "table; rm -rf /",
			wantFormat: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFormat, err := ValidateOutputFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputFormat(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if gotFormat != tt.wantFormat {
				t.Errorf("ValidateOutputFormat(%q) = %v, want %v", tt.input, gotFormat, tt.wantFormat)
			}
		})
	}
}

// TestValidationRejectsAllInjectionPatterns is a comprehensive security test
// that ensures common injection patterns are rejected by all validators.
func TestValidationRejectsAllInjectionPatterns(t *testing.T) {
	injectionPatterns := []string{
		"; rm -rf /",
		"| cat /etc/passwd",
		"` whoami `",
		"`whoami`",
		"$(whoami)",
		"$((1+1))",
		"\n rm -rf /",
		"\r\n del *.*",
		"&& ls",
		"|| ls",
		"> /tmp/file",
		"< /etc/passwd",
		"' OR '1'='1",
		"\" OR \"1\"=\"1",
	}

	validIP := "10.0.0.1"
	validCIDR := "10.0.0.0/24"

	for _, pattern := range injectionPatterns {
		t.Run("IP_"+pattern, func(t *testing.T) {
			input := validIP + pattern
			_, err := ValidateFilterIP(input)
			if err == nil {
				t.Errorf("ValidateFilterIP(%q) should have returned error for injection pattern", input)
			}
		})

		t.Run("CIDR_"+pattern, func(t *testing.T) {
			input := validCIDR + pattern
			_, err := ValidateFilterCIDR(input)
			if err == nil {
				t.Errorf("ValidateFilterCIDR(%q) should have returned error for injection pattern", input)
			}
		})

		t.Run("Output_"+pattern, func(t *testing.T) {
			input := "table" + pattern
			_, err := ValidateOutputFormat(input)
			if err == nil {
				t.Errorf("ValidateOutputFormat(%q) should have returned error for injection pattern", input)
			}
		})
	}
}
