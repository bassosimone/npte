// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// logError logs a message that is always visible (errors, usage hints).
func logError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	style := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	msg = style.Render(strings.TrimRight(msg, "\n"))
	fmt.Fprintln(env.Stderr, msg)
}

// logDetails logs operational progress (suppressible in the future).
func logDetails(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	style := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("8"))
	msg = style.Render(strings.TrimRight(msg, "\n"))
	fmt.Fprintln(env.Stderr, msg)
}

// logCommand logs a command about to be executed.
func logCommand(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	style := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("4"))
	msg = style.Render("+ " + strings.TrimRight(msg, "\n"))
	fmt.Fprintln(env.Stderr, msg)
}
