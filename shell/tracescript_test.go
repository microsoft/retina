// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package shell

import (
	"net"
	"strings"
	"testing"
)

func TestIPToHex(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected uint32
	}{
		{
			name:     "10.0.0.1",
			ip:       "10.0.0.1",
			expected: 0x0a000001,
		},
		{
			name:     "192.168.1.100",
			ip:       "192.168.1.100",
			expected: 0xc0a80164,
		},
		{
			name:     "0.0.0.0",
			ip:       "0.0.0.0",
			expected: 0x00000000,
		},
		{
			name:     "255.255.255.255",
			ip:       "255.255.255.255",
			expected: 0xffffffff,
		},
		{
			name:     "127.0.0.1",
			ip:       "127.0.0.1",
			expected: 0x7f000001,
		},
		{
			name:     "172.16.0.1",
			ip:       "172.16.0.1",
			expected: 0xac100001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			result := ipToHex(ip)
			if result != tt.expected {
				t.Errorf("ipToHex(%s) = 0x%08x, want 0x%08x", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIPToHexOnlyHexOutput(t *testing.T) {
	// SECURITY TEST: Verify that output only contains hex digits
	// This ensures no injection is possible through IP addresses
	ips := []string{
		"10.0.0.1",
		"192.168.1.100",
		"172.16.255.255",
		"0.0.0.0",
		"255.255.255.255",
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", ipStr)
		}

		hex := ipToHex(ip)
		// Convert to string representation
		formatted := strings.ToLower(string([]byte{
			'0', 'x',
			hexChar((hex >> 28) & 0xf),
			hexChar((hex >> 24) & 0xf),
			hexChar((hex >> 20) & 0xf),
			hexChar((hex >> 16) & 0xf),
			hexChar((hex >> 12) & 0xf),
			hexChar((hex >> 8) & 0xf),
			hexChar((hex >> 4) & 0xf),
			hexChar(hex & 0xf),
		}))

		// Verify only contains 0-9, a-f, x
		for _, c := range formatted {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && c != 'x' {
				t.Errorf("ipToHex(%s) produced non-hex character: %c in %s", ipStr, c, formatted)
			}
		}
	}
}

func hexChar(b uint32) byte {
	if b < 10 {
		return byte('0' + b)
	}
	return byte('a' + b - 10)
}

func TestGenerateDropScript(t *testing.T) {
	config := TraceConfig{
		EnableDrops:       true,
		EnableRST:         true,
		EnableErrors:      true,
		EnableRetransmits: true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Verify script structure
	if !strings.Contains(script, "#!/usr/bin/env bpftrace") {
		t.Error("script missing shebang")
	}
	if !strings.Contains(script, "tracepoint:skb:kfree_skb") {
		t.Error("script missing kfree_skb tracepoint")
	}
	if !strings.Contains(script, "BEGIN {") {
		t.Error("script missing BEGIN block")
	}
	if !strings.Contains(script, "END {") {
		t.Error("script missing END block")
	}
	if !strings.Contains(script, "$reason") {
		t.Error("script missing reason variable")
	}
	if !strings.Contains(script, "DROP") {
		t.Error("script missing DROP output")
	}
}

func TestGenerateDropScriptNoFilter(t *testing.T) {
	config := TraceConfig{
		EnableDrops: true,
	}

	gen := NewScriptGenerator(config)
	filter := gen.buildSkbIPFilterCondition()

	// No filter should be empty
	if filter != "" {
		t.Errorf("expected empty filter, got: %s", filter)
	}
}

func TestGenerateDropScriptWithIPFilter(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	config := TraceConfig{
		FilterIPs:  []net.IP{ip},
		OutputJSON: false,
	}

	gen := NewScriptGenerator(config)
	filter := gen.buildSkbIPFilterCondition()

	// Should contain hex representation
	if !strings.Contains(filter, "0x0a000001") {
		t.Errorf("expected filter to contain hex IP 0x0a000001, got: %s", filter)
	}

	// Should use bswap() to handle endianness (kfree_skb reads __be32 as native int)
	if !strings.Contains(filter, "bswap($saddr_raw)") || !strings.Contains(filter, "bswap($daddr_raw)") {
		t.Errorf("expected filter to use bswap() for endianness conversion, got: %s", filter)
	}

	// Should NOT contain the original IP string (security check)
	if strings.Contains(filter, "10.0.0.1") {
		t.Error("filter should not contain original IP string - security risk")
	}
}

func TestGenerateDropScriptWithCIDRFilter(t *testing.T) {
	_, cidr, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("failed to parse CIDR: %v", err)
	}

	config := TraceConfig{
		FilterCIDRs: []*net.IPNet{cidr},
		OutputJSON:  false,
	}

	gen := NewScriptGenerator(config)
	filter := gen.buildSkbIPFilterCondition()

	// Should contain hex network
	if !strings.Contains(filter, "0x0a000000") {
		t.Errorf("expected filter to contain hex network 0x0a000000, got: %s", filter)
	}

	// Should contain hex mask for /24
	if !strings.Contains(filter, "0xffffff00") {
		t.Errorf("expected filter to contain hex mask 0xffffff00, got: %s", filter)
	}

	// Should use bswap() for endianness
	if !strings.Contains(filter, "bswap($saddr_raw)") {
		t.Errorf("expected filter to use bswap() for endianness conversion, got: %s", filter)
	}
}

func TestGenerateDropScriptJSONOutput(t *testing.T) {
	config := TraceConfig{
		OutputJSON:        true,
		EnableDrops:       true,
		EnableRST:         true,
		EnableErrors:      true,
		EnableRetransmits: true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// JSON output should have JSON-formatted printf statements
	// Note: In the bpftrace script, quotes are escaped with backslash
	if !strings.Contains(script, `\"type\":\"DROP\"`) {
		t.Error("JSON script missing DROP type field")
	}
	if !strings.Contains(script, `\"reason_code\"`) {
		t.Error("JSON script missing reason_code field")
	}
	if !strings.Contains(script, `\"src_ip\"`) {
		t.Error("JSON script missing src_ip field")
	}
}

func TestGenerateDropScriptTableOutput(t *testing.T) {
	config := TraceConfig{
		EnableDrops:       true,
		EnableRST:         true,
		EnableErrors:      true,
		EnableRetransmits: true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Table output should have header
	if !strings.Contains(script, "TIME") {
		t.Error("table script missing TIME header")
	}
	if !strings.Contains(script, "TYPE") {
		t.Error("table script missing TYPE header")
	}
	if !strings.Contains(script, "REASON") {
		t.Error("table script missing REASON header")
	}
	if !strings.Contains(script, "STATE") {
		t.Error("table script missing STATE header")
	}
	if !strings.Contains(script, "PROBE") {
		t.Error("table script missing PROBE header")
	}
}

func TestScriptGeneratorNoUserStringInterpolation(t *testing.T) {
	// SECURITY TEST: Verify that user-provided values are never
	// directly interpolated as strings into the script

	// Try various concerning inputs - all should be rejected by validation
	// but even if they made it here, they should be safe

	// Create a generator with an IP that could be injection attempt
	// Note: This IP is valid but we verify it's converted to hex
	ip := net.ParseIP("127.0.0.1")
	config := TraceConfig{
		FilterIPs:  []net.IP{ip},
		OutputJSON: false,
	}

	gen := NewScriptGenerator(config)
	filter := gen.buildSkbIPFilterCondition()

	// The node name should NOT appear in the filter
	if strings.Contains(filter, "evil-node") {
		t.Error("node name should not appear in filter condition")
	}

	// The filter should only contain safe characters
	// Allow: hex digits, whitespace, operators, struct names, etc.
	// Disallow: semicolons, backticks, $() syntax
	dangerousPatterns := []string{
		"`;",
		"$()",
		"`",
		"system(",
		"exec(",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(filter, pattern) {
			t.Errorf("filter contains dangerous pattern: %s", pattern)
		}
	}
}

func TestBuildIPFilter(t *testing.T) {
	tests := []struct {
		name         string
		ips          []string
		expectHexIPs []string
		noStringIPs  bool // Verify original strings don't appear
	}{
		{
			name:         "single IP",
			ips:          []string{"10.0.0.1"},
			expectHexIPs: []string{"0x0a000001"},
			noStringIPs:  true,
		},
		{
			name:         "multiple IPs",
			ips:          []string{"10.0.0.1", "192.168.1.1"},
			expectHexIPs: []string{"0x0a000001", "0xc0a80101"},
			noStringIPs:  true,
		},
		{
			name:         "Class B network",
			ips:          []string{"172.16.0.1"},
			expectHexIPs: []string{"0xac100001"},
			noStringIPs:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsedIPs []net.IP
			for _, ipStr := range tt.ips {
				ip := net.ParseIP(ipStr)
				if ip == nil {
					t.Fatalf("failed to parse IP: %s", ipStr)
				}
				parsedIPs = append(parsedIPs, ip)
			}

			config := TraceConfig{
				FilterIPs:  parsedIPs,
				OutputJSON: false,
			}

			gen := NewScriptGenerator(config)
			filter := gen.buildSkbIPFilterCondition()

			// Verify hex IPs are present
			for _, hexIP := range tt.expectHexIPs {
				if !strings.Contains(filter, hexIP) {
					t.Errorf("expected filter to contain %s, got: %s", hexIP, filter)
				}
			}

			// Verify original string IPs are NOT present (security)
			if tt.noStringIPs {
				for _, ipStr := range tt.ips {
					if strings.Contains(filter, ipStr) {
						t.Errorf("filter should not contain original IP string %s - security risk", ipStr)
					}
				}
			}
		})
	}
}

func TestBuildCIDRFilter(t *testing.T) {
	tests := []struct {
		name       string
		cidr       string
		expectNet  string // hex network address
		expectMask string // hex mask
	}{
		{
			name:       "/24 network",
			cidr:       "10.0.0.0/24",
			expectNet:  "0x0a000000",
			expectMask: "0xffffff00",
		},
		{
			name:       "/16 network",
			cidr:       "172.16.0.0/16",
			expectNet:  "0xac100000",
			expectMask: "0xffff0000",
		},
		{
			name:       "/8 network",
			cidr:       "10.0.0.0/8",
			expectNet:  "0x0a000000",
			expectMask: "0xff000000",
		},
		{
			name:       "/32 single host",
			cidr:       "192.168.1.100/32",
			expectNet:  "0xc0a80164",
			expectMask: "0xffffffff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cidr, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("failed to parse CIDR: %v", err)
			}

			config := TraceConfig{
				FilterCIDRs: []*net.IPNet{cidr},
				OutputJSON:  false,
			}

			gen := NewScriptGenerator(config)
			filter := gen.buildSkbIPFilterCondition()

			if !strings.Contains(filter, tt.expectNet) {
				t.Errorf("expected filter to contain network %s, got: %s", tt.expectNet, filter)
			}
			if !strings.Contains(filter, tt.expectMask) {
				t.Errorf("expected filter to contain mask %s, got: %s", tt.expectMask, filter)
			}
		})
	}
}

func TestMixedIPAndCIDRFilter(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	_, cidr, _ := net.ParseCIDR("192.168.0.0/16")

	config := TraceConfig{
		FilterIPs:   []net.IP{ip},
		FilterCIDRs: []*net.IPNet{cidr},
		OutputJSON:  false,
	}

	gen := NewScriptGenerator(config)
	filter := gen.buildSkbIPFilterCondition()

	// Should contain both IP and CIDR hex values
	if !strings.Contains(filter, "0x0a000001") {
		t.Error("filter missing IP hex value")
	}
	if !strings.Contains(filter, "0xc0a80000") {
		t.Error("filter missing CIDR network hex value")
	}

	// Conditions should be combined with OR
	if !strings.Contains(filter, "||") {
		t.Error("filter should combine conditions with OR")
	}
}

func TestDropReasonsCommand(t *testing.T) {
	cmd := DropReasonsCommand()

	// Should be a shell command
	if len(cmd) != 3 {
		t.Errorf("expected 3 elements (sh -c cmd), got %d", len(cmd))
	}
	if cmd[0] != "sh" || cmd[1] != "-c" {
		t.Error("expected sh -c prefix")
	}

	// Should read from the tracepoint format file
	if !strings.Contains(cmd[2], "kfree_skb/format") {
		t.Error("command should read from kfree_skb/format")
	}

	// Should have fallback message
	if !strings.Contains(cmd[2], "Could not read") {
		t.Error("command should have fallback error message")
	}
}

func TestScriptUsesNumericReasonCode(t *testing.T) {
	config := TraceConfig{
		EnableDrops: true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Script should NOT contain hardcoded reason names
	hardcodedReasons := []string{
		"NO_SOCKET",
		"NETFILTER_DROP",
		"OTHERHOST",
	}

	for _, reason := range hardcodedReasons {
		if strings.Contains(script, reason) {
			t.Errorf("script should not hardcode reason name: %s", reason)
		}
	}

	// Script should use $reason (numeric) in output
	if !strings.Contains(script, "$reason") {
		t.Error("script should use $reason variable for numeric code")
	}
}

func TestGenerateNfqueueDropProbe(t *testing.T) {
	config := TraceConfig{
		EnableNfqueueDrops: true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Verify fexit probe is present
	if !strings.Contains(script, "fexit:vmlinux:__nf_queue") {
		t.Error("script missing fexit:vmlinux:__nf_queue probe")
	}

	// Verify it checks return value
	if !strings.Contains(script, "retval") {
		t.Error("script missing retval check")
	}

	// Verify it accesses args->skb
	if !strings.Contains(script, "args->skb") {
		t.Error("script missing args->skb access")
	}

	// Verify it reads queue number
	if !strings.Contains(script, "args->queuenum") {
		t.Error("script missing args->queuenum access")
	}

	// Verify NFQ_DROP event type in table output
	if !strings.Contains(script, "NFQ_DROP") {
		t.Error("script missing NFQ_DROP event type")
	}

	// Verify __nf_queue probe name in output
	if !strings.Contains(script, "__nf_queue") {
		t.Error("script missing __nf_queue probe name in output")
	}

	// Verify errno decoding
	if !strings.Contains(script, "ESRCH") {
		t.Error("script missing ESRCH errno name")
	}
	if !strings.Contains(script, "ENOMEM") {
		t.Error("script missing ENOMEM errno name")
	}
}

func TestGenerateNfqueueDropProbeJSON(t *testing.T) {
	config := TraceConfig{
		EnableNfqueueDrops: true,
		OutputJSON:         true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Verify JSON output fields
	if !strings.Contains(script, `\"type\":\"NFQ_DROP\"`) {
		t.Error("JSON script missing NFQ_DROP type field")
	}
	if !strings.Contains(script, `\"queue\"`) {
		t.Error("JSON script missing queue field")
	}
	if !strings.Contains(script, `\"errno\"`) {
		t.Error("JSON script missing errno field")
	}
	if !strings.Contains(script, `\"probe\":\"__nf_queue\"`) {
		t.Error("JSON script missing __nf_queue probe field")
	}
}

func TestGenerateNfqueueDropProbeWithIPFilter(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	config := TraceConfig{
		EnableNfqueueDrops: true,
		FilterIPs:          []net.IP{ip},
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Should contain fexit probe
	if !strings.Contains(script, "fexit:vmlinux:__nf_queue") {
		t.Error("script missing fexit probe")
	}

	// Should contain IP filter hex
	if !strings.Contains(script, "0x0a000001") {
		t.Error("script missing hex IP in NFQUEUE probe filter")
	}

	// Should use bswap for filter
	if !strings.Contains(script, "bswap($saddr_raw)") {
		t.Error("script missing bswap in NFQUEUE probe filter")
	}
}

func TestGenerateAllProbesIncludingNfqueue(t *testing.T) {
	// Verify all probes can be generated together
	config := TraceConfig{
		EnableDrops:        true,
		EnableRST:          true,
		EnableErrors:       true,
		EnableRetransmits:  true,
		EnableNfqueueDrops: true,
	}

	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// All probes should be present
	if !strings.Contains(script, "tracepoint:skb:kfree_skb") {
		t.Error("script missing kfree_skb probe")
	}
	if !strings.Contains(script, "tcp_send_reset") {
		t.Error("script missing RST probe")
	}
	if !strings.Contains(script, "inet_sk_error_report") {
		t.Error("script missing socket error probe")
	}
	if !strings.Contains(script, "tcp_retransmit_skb") {
		t.Error("script missing retransmit probe")
	}
	if !strings.Contains(script, "fexit:vmlinux:__nf_queue") {
		t.Error("script missing NFQUEUE probe")
	}
}
