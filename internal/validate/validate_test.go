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
