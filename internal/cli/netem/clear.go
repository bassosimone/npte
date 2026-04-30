// SPDX-License-Identifier: GPL-3.0-or-later

package netem

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/registry"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// clearMain is the main of the `netem clear` subcommand.
func clearMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netem clear", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Remove the root qdisc from <if> inside <ns>. Deleting the root also "+
			"deletes any child qdisc attached at parent 1:, so this undoes "+
			"`npte netem apply` regardless of whether --child was used.",
		"Idempotent: if there is no root qdisc to remove (e.g. this command "+
			"has already been run, or `apply` was never called), the error "+
			"from tc is tolerated.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<ns> <if>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	// NOPASSWD audit invariant: this command is part of the set that
	// `npte sudoers` allowlists for sudo execution without a password
	// (see CLAUDE.md in this package). Every flag value, positional, or
	// environment value forwarded to a subprocess must be validated
	// here — fail loud, prefer hardcoded literals, never trust the
	// caller's bytes. A missing check is a passwordless privesc hole.

	ns := fset.Args()[0]
	iface := fset.Args()[1]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netem clear: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.IfaceName(iface); err != nil {
		logx.Error("npte netem clear: %s", err)
		env.Exit(2)
		return nil
	}

	unlock := registry.MustLock(ctx, env, dryRun)
	defer unlock()

	if err := registry.RequireManaged(env, dryRun, ns); err != nil {
		logx.Error("npte netem clear: %s", err)
		env.Exit(2)
		return nil
	}

	logx.Details("npte: remove root qdisc on %q inside %q (may already be absent)", iface, ns)
	subprocess.MustRunTolerant(ctx, dryRun, "ip", "netns", "exec", ns, "tc", "qdisc", "del", "dev", iface, "root")

	logx.Details("npte: shaping cleared on %q inside %q", iface, ns)
	return nil
}
