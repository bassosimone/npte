// SPDX-License-Identifier: GPL-3.0-or-later

package validate

import (
	"strings"
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

// The netns-name cap exists so that "if-<ns>" fits IFNAMSIZ; pin the
// relationship so bumping one limit without the other fails loud.
func TestNetnsNameFitsIfaceName(t *testing.T) {
	longest := strings.Repeat("a", netnsNameMaxLen)
	assert.NoError(t, NetnsName(longest))
	assert.NoError(t, IfaceName("if-"+longest))
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
		{"leading hyphen ipv4", "-10.0.1.1", "ip address"},
		{"leading hyphen ipv6", "-2001:db8::1", "ip address"},
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

func TestDebootstrapSuite(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty", "", "empty"},
		{"noble", "noble", ""},
		{"bookworm", "bookworm", ""},
		{"trixie", "trixie", ""},
		{"alnum", "buster10", ""},
		{"leading digit", "1noble", "must match"},
		{"uppercase", "Noble", "must match"},
		{"hyphen", "noble-updates", "must match"},
		{"underscore", "noble_x", "must match"},
		{"shell metachar", "noble;rm", "must match"},
		{"slash", "noble/x", "must match"},
		{"space", "no ble", "must match"},
		{"leading hyphen", "-noble", "must match"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := DebootstrapSuite(tc.input)
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
		{"leading hyphen ipv4", "-10.0.1.0/24", "cidr"},
		{"leading hyphen ipv6", "-2001:db8::/64", "cidr"},
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
