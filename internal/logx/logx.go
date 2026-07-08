// SPDX-License-Identifier: GPL-3.0-or-later

// Package logx implements logging.
package logx

import (
	"fmt"
	"strings"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/charmbracelet/lipgloss"
)

func log(color string, format string, args ...any) {
	current := testable.Env
	msg := fmt.Sprintf(format, args...)
	style := current.LogRenderer.NewStyle().Foreground(lipgloss.Color(color))
	msg = style.Render(strings.TrimRight(msg, "\n"))
	// TODO(bassosimone): panicking on a failed log write is questionable:
	// the failure is unreportable (the report channel is the broken thing)
	// and the alternative to continuing is aborting a possibly half-done
	// kernel mutation because the user's pager exited. In production the
	// panic is mostly moot anyway: on EPIPE for fd 2 the Go runtime
	// re-raises SIGPIPE and the process dies silently before Fprintln
	// returns, so this only fires for non-fd-2 writers (e.g. test mocks).
	// Genuinely surviving `npte ... 2>&1 | head` would require ignoring
	// SIGPIPE at startup in addition to swallowing the error here.
	_ = runtimex.PanicOnError1(fmt.Fprintln(current.Stderr, msg))
}

// Error logs an message that is always visible such as errors and usage hints.
func Error(format string, args ...any) {
	log("1", format, args...)
}

// Details logs potentially suppressible operational progress.
func Details(format string, args ...any) {
	log("8", format, args...)
}

// Command logs the command line that we're about to exec.
func Command(format string, args ...any) {
	log("4", "+ "+format, args...)
}
