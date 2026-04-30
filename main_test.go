// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/bassosimone/deferexit"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/testenv"
	"github.com/stretchr/testify/assert"
)

// Simple smoke test ensuring that `npte --help` does not fail.
func Test_main(t *testing.T) {
	stubs := testenv.Setup(t)
	env := testable.Env
	env.Args = []string{"npte", "--help"}
	main()
	assert.Equal(t, -1, stubs.ExitCode) // exit not called
	assert.Contains(t, stubs.Stdout.String(), "Network Performance Testing Environment")
}

// runRealMain wires up testenv, sets env.Args and an exiting env.Exit
// (the testenv stub only records the code and returns, but vclip expects
// Exit to terminate, so we use deferexit.Panic which deferexit.Run can catch),
// then invokes realMain and returns the captured stubs and exit code.
func runRealMain(t *testing.T, argv ...string) (*testenv.Stubs, int) {
	stubs := testenv.Setup(t)
	testable.Env.Args = append([]string{"npte"}, argv...)
	testable.Env.Exit = deferexit.Panic
	return stubs, deferexit.Run(realMain)
}

// Test that ensures an unknown command causes exit(2).
func Test_realMain_UnknownCommand(t *testing.T) {
	stubs, code := runRealMain(t, "unknown-command")
	assert.Equal(t, 2, code)
	assert.Contains(t, stubs.Stderr.String(), "command not found")
}

// Test that ensures an unknown subcommand causes exit(2).
func Test_realMain_UnknownSubcommand(t *testing.T) {
	stubs, code := runRealMain(t, "netem", "unknown-subcommand")
	assert.Equal(t, 2, code)
	assert.Contains(t, stubs.Stderr.String(), "command not found")
}
