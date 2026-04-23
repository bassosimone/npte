// SPDX-License-Identifier: GPL-3.0-or-later

// Package star implements the star subcommand.
package star

import (
	"context"
	"os"
	"os/exec"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

// Main is the main of the star subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte star", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Compose a fixed three-node star topology built entirely from the other "+
			"`npte` primitives. The leaves are named `client` and `server`; the hub "+
			"is named `router`. The router is also an internet gateway, so that both "+
			"leaves have working off-link connectivity in one step.",
		"This is batteries-included convenience: the exact sequence of `npte netns`, "+
			"`npte netns add-route`, and `npte gateway create` invocations it performs "+
			"is visible with `--dry-run`. For non-default names, addresses, or "+
			"topologies, call the underlying primitives directly.",
	)
	disp.AddCommand("create", vclip.CommandFunc(createMain), "Create the `client`/`router`/`server` star.")
	disp.AddCommand("destroy", vclip.CommandFunc(destroyMain), "Destroy the `client`/`router`/`server` star.")

	return disp.Main(ctx, args)
}

// runSelf executes the current `npte` binary with the given arguments,
// so that `npte star` can compose other `npte` subcommands without
// duplicating their logic.
//
// The binary path is resolved once by the caller via [os.Executable],
// which returns an absolute path. This is robust against a relative
// argv[0] (e.g. when invoked as "./npte") and against callers that
// alter PATH between invocations.
//
// stdin/stdout/stderr are inherited from [testable.Env], so a child in
// `--dry-run` mode prints its shell script directly to our stdout and
// logs (on stderr) interleave naturally with our own.
func runSelf(ctx context.Context, self string, args ...string) {
	env := testable.Env
	quoted := shellquote.Join(append([]string{self}, args...)...)
	logx.Command("%s", quoted)
	cmd := exec.CommandContext(ctx, self, args...)
	cmd.Stdin = env.Stdin
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	env.LogFatalOnError0(env.RunCommand(cmd))
}

// selfPath returns the absolute path to the currently running binary,
// or logs and exits on failure. Split out so that create/destroy share
// the same error-handling shape.
func selfPath(who string) string {
	self, err := os.Executable()
	if err != nil {
		logx.Error("%s: cannot resolve own executable path: %s", who, err)
		testable.Env.Exit(1)
	}
	return self
}
