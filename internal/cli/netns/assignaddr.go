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

// assignAddrMain is the main of the `netns assign-addr` subcommand.
func assignAddrMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns assign-addr", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Assigns an IP address to an interface inside a network namespace. "+
			"The <cidr> argument must include a prefix length; both IPv4 and "+
			"IPv6 use the same syntax, so invoke this command twice for "+
			"dual-stack configurations.",
		"The kernel auto-installs the connected route for the prefix, which "+
			"is enough for peer reachability on the link. Default routes and "+
			"other policy routing are a separate concern.",
		"The namespace must already exist (see `npte netns create`) and the "+
			"interface must already be present inside it (see `npte netns connect`).",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<ns> <iface> <cidr>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 3
	fset.MaxPositionalArgs = 3
	runtimex.PanicOnError0(fset.Parse(args))

	// NOPASSWD audit invariant: this command is part of the set that
	// `npte sudoers` allowlists for sudo execution without a password
	// (see CLAUDE.md in this package). Every flag value, positional, or
	// environment value forwarded to a subprocess must be validated
	// here — fail loud, prefer hardcoded literals, never trust the
	// caller's bytes. A missing check is a passwordless privesc hole.

	ns := fset.Args()[0]
	iface := fset.Args()[1]
	cidr := fset.Args()[2]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netns assign-addr: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.IfaceName(iface); err != nil {
		logx.Error("npte netns assign-addr: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.CIDR(cidr); err != nil {
		logx.Error("npte netns assign-addr: %s", err)
		env.Exit(2)
		return nil
	}

	logx.Details("npte: assign %s to %q inside %q", cidr, iface, ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns, "ip", "addr", "add", cidr, "dev", iface)

	logx.Details("npte: %s is configured on %q @ %s", cidr, iface, ns)
	return nil
}
