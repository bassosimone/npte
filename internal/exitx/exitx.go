// SPDX-License-Identifier: GPL-3.0-or-later

// Package exitx implements panic-based program exit so that deferred
// cleanup runs before the process actually terminates.
//
// Code that wants to abort the program calls [Do] instead of os.Exit.
// The outermost main function installs `defer Recover(os.Exit)`, which
// catches the typed panic raised by [Do] and converts it back to a
// real process exit after every deferred function has run.
package exitx

// Exit is the typed value carried by panics raised via [Do].
type Exit struct {
	Code int
}

// Do panics with an [Exit] value carrying code. Use this instead of
// os.Exit so that deferred functions (lockfile release, marker
// cleanup, etc.) run before the process terminates.
func Do(code int) {
	panic(Exit{Code: code})
}

// Recover catches an [Exit] panic raised via [Do] and calls onExit
// with the carried code. Any other panic value is re-raised so that
// real programmer bugs still produce the standard Go panic output.
//
// Use it as `defer exitx.Recover(os.Exit)` at the top of main.
func Recover(onExit func(code int)) {
	r := recover()
	if r == nil {
		return
	}
	if e, ok := r.(Exit); ok {
		onExit(e.Code)
		return
	}
	panic(r)
}
