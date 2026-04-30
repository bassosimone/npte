// SPDX-License-Identifier: GPL-3.0-or-later

// Package exitx implements panic-based program exit so that deferred
// cleanup runs before the process actually terminates.
//
// Code that wants to abort the program calls [Panic] instead of os.Exit.
// The outermost main function installs `defer Recover(os.Exit)`, which
// catches the typed panic raised by [Panic] and converts it back to a
// real process exit after every deferred function has run.
//
// [Run] is a convenience wrapper that runs a function and returns the
// exit code carried by any [Panic] raised inside it (or zero if the
// function returns normally). It is intended for tests that want to
// assert on the exit code that a main-like function would have
// produced.
//
// Production main should NOT be written as `os.Exit(Run(realMain))`:
// that pattern calls os.Exit unconditionally even when realMain returns
// normally, which makes the success path of main untestable (any test
// that calls main would terminate the test binary). The recommended
// production pattern is:
//
//	func main() {
//		defer Recover(os.Exit)
//		realMain()
//	}
//
// which only reaches os.Exit when [Panic] was actually raised.
package exitx

// Code is the exit code carried by panics raised via [Panic].
type Code int

// Panic panics with a [Code] value carrying the given exit code. Use
// this instead of os.Exit so that deferred functions (lockfile
// release, marker cleanup, etc.) run before the process terminates.
func Panic(code int) {
	panic(Code(code))
}

// Recover catches a [Code] panic raised via [Panic] and calls onExit
// with the carried code. Any other panic value is re-raised so that
// real programmer bugs still produce the standard Go panic output.
//
// Use it as `defer exitx.Recover(os.Exit)` at the top of main.
func Recover(onExit func(code int)) {
	r := recover()
	if r == nil {
		return
	}
	if c, ok := r.(Code); ok {
		onExit(int(c))
		return
	}
	panic(r)
}

// Run runs fx and returns the exit code carried by any [Panic] raised
// inside fx, or zero if fx returned normally. Non-[Code] panics are
// re-raised so real bugs are not swallowed.
//
// Use this in tests to assert on the exit code that a main-like
// function would have produced. See the package documentation for
// why production main should not be written as `os.Exit(Run(...))`.
func Run(fx func()) (exitcode int) {
	defer Recover(func(code int) {
		exitcode = code
	})
	fx()
	return
}
