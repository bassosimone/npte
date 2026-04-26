// SPDX-License-Identifier: GPL-3.0-or-later

package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChildQdiscKind(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"cake", "cake", ""},
		{"empty", "", "not allowed"},
		{"unknown", "drr", "not allowed"},
		{"classful htb", "htb", "not allowed"},
		{"netem self", "netem", "not allowed"},
		{"uppercase", "CAKE", "not allowed"},
		{"with space", "cake ", "not allowed"},
		{"shell metachar", "cake;rm", "not allowed"},
		// Previously-allowed kinds that are now off-list until they
		// have a concrete use case + per-kind flags.
		{"fq_codel removed", "fq_codel", "not allowed"},
		{"bfifo removed", "bfifo", "not allowed"},
		{"pfifo removed", "pfifo", "not allowed"},
		{"sfq removed", "sfq", "not allowed"},
		{"red removed", "red", "not allowed"},
		{"codel removed", "codel", "not allowed"},
		{"pie removed", "pie", "not allowed"},
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
