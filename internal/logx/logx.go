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
