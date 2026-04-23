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

