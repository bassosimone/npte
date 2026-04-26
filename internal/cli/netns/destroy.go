// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// destroyMain is the main of the `netns destroy` subcommand.
func destroyMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns destroy", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Undoes `npte netns create`: removes the namespace-scoped "+
			"/etc/netns/<name>/ directory and then destroys the kernel "+
			"network namespace. Destroying the namespace cascades to the "+
			"interfaces inside it, the peer ends of any attached veth pairs, "+
			"and the per-namespace iptables state.",
		"Does NOT touch host-side gateway state (e.g. MASQUERADE rules in "+
			"the root netns). Gateway management is a separate concern; use "+
			"`npte gateway destroy` to tear down those rules.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<name>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	// NOPASSWD audit invariant: this command is part of the set that
	// `npte sudoers` allowlists for sudo execution without a password
	// (see CLAUDE.md in this package). Every flag value, positional, or
	// environment value forwarded to a subprocess must be validated
	// here — fail loud, prefer hardcoded literals, never trust the
	// caller's bytes. A missing check is a passwordless privesc hole.

	ns := fset.Args()[0]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netns destroy: %s", err)
		env.Exit(2)
		return nil
	}

	logx.Details("npte: remove /etc/netns/%s", ns)
	subprocess.MustRun(ctx, dryRun, "rm", "-rf", "/etc/netns/"+ns)

	logx.Details("npte: destroy namespace %q", ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "del", ns)

	logx.Details("npte: namespace %q is gone", ns)
	return nil
}
