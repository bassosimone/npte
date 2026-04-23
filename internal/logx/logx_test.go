// SPDX-License-Identifier: GPL-3.0-or-later

package logx

import (
	"bytes"
	"testing"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// setup swaps [testable.Env] with an instance that writes to a buffer
// and renders without ANSI color codes. Returns the buffer and restores
// the previous environment via [t.Cleanup].
func setup(t *testing.T) *bytes.Buffer {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	saved := testable.Env
	testable.Env = &testable.Environ{
		Stderr:      &buf,
		LogRenderer: lipgloss.NewRenderer(&buf),
	}
	t.Cleanup(func() { testable.Env = saved })
	return &buf
}

func TestLog(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string, ...any)
		fmt  string
		args []any
		want string
	}{
		{"Error", Error, "oops %d", []any{42}, "oops 42\n"},
		{"Details", Details, "step %s", []any{"a"}, "step a\n"},
		{"Command", Command, "ip %s", []any{"link"}, "+ ip link\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := setup(t)
			tt.fn(tt.fmt, tt.args...)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

func TestLog_TrimsTrailingNewlines(t *testing.T) {
	buf := setup(t)
	Error("boom\n\n")
	assert.Equal(t, "boom\n", buf.String())
}
