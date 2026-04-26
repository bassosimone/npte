// SPDX-License-Identifier: GPL-3.0-or-later

package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetnsName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string // empty means no error; otherwise substring match
	}{
		{"empty", "", "empty"},
		{"single letter", "a", ""},
		{"max length", "abcdefghijkl", ""},
		{"too long", "abcdefghijklm", "exceeds 12"},
		{"leading digit", "1abc", "must match"},
		{"uppercase", "Abc", "must match"},
		{"hyphen", "a-b", "must match"},
		{"underscore", "a_b", "must match"},
		{"alnum", "a1b2c3d", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetnsName(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestIfaceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty", "", "empty"},
		{"single letter", "a", ""},
		{"typical", "eth0", ""},
		{"hyphen", "if-alice", ""},
		{"underscore", "my_iface", ""},
		{"dot", "vlan.10", ""},
		{"uppercase", "Eth0", ""},
		{"max length", "abcdefghijklmno", ""},
		{"too long", "abcdefghijklmnop", "exceeds 15"},
		{"dot reserved", ".", "reserved"},
		{"dotdot reserved", "..", "reserved"},
		{"leading hyphen", "-eth0", "must not start with a hyphen"},
		{"slash", "a/b", "forbidden"},
		{"colon", "a:b", "forbidden"},
		{"space", "a b", "forbidden"},
		{"tab", "a\tb", "forbidden"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IfaceName(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestIPAddr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"ipv4", "10.0.1.1", ""},
		{"ipv4 zero", "0.0.0.0", ""},
		{"ipv6", "2001:db8::1", ""},
		{"ipv6 zero", "::", ""},
		{"ipv6 loopback", "::1", ""},
		{"with prefix", "10.0.1.0/24", "ip address"},
		{"garbage", "not-an-ip", "ip address"},
		{"empty", "", "ip address"},
		{"octet out of range", "10.0.1.256", "ip address"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IPAddr(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty", "", "empty"},
		{"typical", "alice", ""},
		{"underscore start", "_alice", ""},
		{"hyphen middle", "al-ice", ""},
		{"digit middle", "alice1", ""},
		{"max length", "abcdefghijklmnopqrstuvwxyz012345", ""},
		{"too long", "abcdefghijklmnopqrstuvwxyz0123456", "exceeds 32"},
		{"leading digit", "1alice", "must match"},
		{"leading hyphen", "-alice", "must match"},
		{"uppercase", "Alice", "must match"},
		{"shell metachar semicolon", "alice;rm", "must match"},
		{"shell metachar dollar", "alice$x", "must match"},
		{"space", "al ice", "must match"},
		{"slash", "al/ice", "must match"},
		{"dot", "al.ice", "must match"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Username(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestEnvVarName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty", "", "empty"},
		{"uppercase", "FOO", ""},
		{"lowercase", "foo", ""},
		{"mixed case with digits and underscore", "FooBar_42", ""},
		{"underscore start", "_FOO", ""},
		{"single underscore", "_", ""},
		{"leading digit", "1FOO", "must match"},
		{"leading hyphen", "-FOO", "must match"},
		{"hyphen middle", "FOO-BAR", "must match"},
		{"shell metachar semicolon", "FOO;rm", "must match"},
		{"shell metachar dollar", "FOO$X", "must match"},
		{"space", "FOO BAR", "must match"},
		{"slash", "FOO/BAR", "must match"},
		{"dot", "FOO.BAR", "must match"},
		{"equals", "FOO=BAR", "must match"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := EnvVarName(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestChildQdiscKind(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"fq_codel", "fq_codel", ""},
		{"cake", "cake", ""},
		{"bfifo", "bfifo", ""},
		{"pfifo", "pfifo", ""},
		{"sfq", "sfq", ""},
		{"red", "red", ""},
		{"codel", "codel", ""},
		{"pie", "pie", ""},
		{"empty", "", "not allowed"},
		{"unknown", "drr", "not allowed"},
		{"classful htb", "htb", "not allowed"},
		{"netem self", "netem", "not allowed"},
		{"uppercase", "FQ_CODEL", "not allowed"},
		{"with space", "fq_codel ", "not allowed"},
		{"shell metachar", "fq_codel;rm", "not allowed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ChildQdiscKind(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNetemNoKnobSmuggling(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		wantErr string
	}{
		{"empty", nil, ""},
		{"plain delay", []string{"10ms"}, ""},
		{"delay with jitter", []string{"10ms", "2ms"}, ""},
		{"delay with distribution", []string{"10ms", "2ms", "distribution", "paretonormal"}, ""},
		{"loss random", []string{"random", "1%"}, ""},
		{"loss gemodel", []string{"gemodel", "0.1", "0.05", "0.9", "0.95"}, ""},
		{"rate with overheads", []string{"10mbit", "1000", "500"}, ""},
		{"slot with packets", []string{"5ms", "10ms", "packets", "64"}, ""},
		{"smuggle loss in delay", []string{"10ms", "loss", "random", "100%"}, "knob"},
		{"smuggle ecn", []string{"1%", "ecn"}, "knob"},
		{"smuggle rate", []string{"10ms", "rate", "1mbit"}, "knob"},
		{"smuggle delay", []string{"delay", "10ms"}, "knob"},
		{"smuggle reorder", []string{"reorder", "25%"}, "knob"},
		{"smuggle corrupt", []string{"corrupt", "1%"}, "knob"},
		{"smuggle duplicate", []string{"duplicate", "1%"}, "knob"},
		{"smuggle gap", []string{"gap", "10"}, "knob"},
		{"smuggle limit", []string{"limit", "1000"}, "knob"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NetemNoKnobSmuggling(tc.tokens)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"ipv4 network", "10.0.1.0/24", ""},
		{"ipv4 host bits set", "10.0.1.1/24", ""},
		{"ipv4 host route", "10.0.1.1/32", ""},
		{"ipv6 network", "2001:db8::/64", ""},
		{"ipv6 host bits set", "2001:db8::1/64", ""},
		{"ipv6 host route", "2001:db8::1/128", ""},
		{"bare ipv4", "10.0.1.1", "cidr"},
		{"bare ipv6", "2001:db8::1", "cidr"},
		{"empty", "", "cidr"},
		{"garbage", "not-a-cidr", "cidr"},
		{"bad prefix len v4", "10.0.1.0/33", "cidr"},
		{"bad prefix len v6", "2001:db8::/129", "cidr"},
		{"negative prefix", "10.0.1.0/-1", "cidr"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CIDR(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
