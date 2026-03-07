// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "fmt"

// logAlways logs a message that is always visible (errors, usage hints).
func logAlways(format string, args ...any) {
	fmt.Fprintf(env.Stderr, format, args...)
}

// logDetails logs operational progress (suppressible in the future).
func logDetails(format string, args ...any) {
	fmt.Fprintf(env.Stderr, format, args...)
}
