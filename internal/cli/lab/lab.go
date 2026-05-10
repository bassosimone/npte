// SPDX-License-Identifier: GPL-3.0-or-later

// Package lab implements the lab subcommand.
package lab

import (
	"context"
	"os/exec"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

// Main is the main of the lab subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte lab", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Compose a fixed three-node `client — router — server` lab built entirely "+
			"from the other `npte` primitives. The leaves are named `client` and "+
			"`server`; the middle node is named `router`. The leaves can talk to "+
			"each other through the router; the lab itself does not include a host "+
			"uplink, so off-link connectivity requires layering `npte gateway "+
			"create router 172.16.1.0/24 <ext-iface>` on top.",
		"This is batteries-included convenience: the exact sequence of `npte netns` "+
			"invocations it performs is visible with `--dry-run`. For non-default "+
			"names, addresses, or topologies, call the underlying primitives "+
			"directly.",
		"Because `lab` only composes `netns` primitives — no host-namespace "+
			"state — it is safe to run under the same NOPASSWD sudoers allowlist "+
			"as `netns` and `netem`. See `npte sudoers` for the snippet.",
	)
	disp.AddCommand("create", vclip.CommandFunc(createMain), "Create the `client`/`router`/`server` lab.")
	disp.AddCommand("destroy", vclip.CommandFunc(destroyMain), "Destroy the `client`/`router`/`server` lab.")
	disp.AddCommand("netem", vclip.CommandFunc(netemMain), "Shape the lab's access link with a named profile.")

	return disp.Main(ctx, args)
}

// runSelf executes the current `npte` binary with the given arguments,
// so that `npte lab` can compose other `npte` subcommands without
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
// the same error-handling shape. The lookup is routed through
// [testable.Env.Executable] so tests can stub it.
func selfPath(who string) string {
	self, err := testable.Env.Executable()
	if err != nil {
		logx.Error("%s: cannot resolve own executable path: %s", who, err)
		testable.Env.Exit(1)
	}
	return self
}
