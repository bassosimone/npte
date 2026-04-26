// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"net/netip"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// addRouteMain is the main of the `netns add-route` subcommand.
func addRouteMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns add-route", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Adds a route inside a network namespace. The <dest> argument is "+
			"either the literal \"default\" or a CIDR prefix (IPv4 or IPv6). "+
			"The <via> argument is a bare next-hop IP address.",
		"When <dest> is a CIDR, its address family must match <via>. When "+
			"<dest> is \"default\", the family is inferred from <via>, so a "+
			"default route can be installed for either family.",
		"The namespace must already exist (see `npte netns create`) and <via> "+
			"must be reachable on a directly connected interface inside it "+
			"(see `npte netns assign-addr`).",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<ns> <dest> <via>"
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
	dest := fset.Args()[1]
	via := fset.Args()[2]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netns add-route: %s", err)
		env.Exit(2)
		return nil
	}
	if dest != "default" {
		if err := validate.CIDR(dest); err != nil {
			logx.Error("npte netns add-route: %s", err)
			env.Exit(2)
			return nil
		}
	}
	if err := validate.IPAddr(via); err != nil {
		logx.Error("npte netns add-route: %s", err)
		env.Exit(2)
		return nil
	}

	if dest != "default" {
		destPrefix := netip.MustParsePrefix(dest)
		viaAddr := netip.MustParseAddr(via)
		if destPrefix.Addr().Is4() != viaAddr.Is4() {
			logx.Error("npte netns add-route: dest %q and via %q disagree on address family", dest, via)
			env.Exit(2)
			return nil
		}
	}

	logx.Details("npte: add route to %s via %s inside %q", dest, via, ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns, "ip", "route", "add", dest, "via", via)

	logx.Details("npte: route to %s via %s is installed in %q", dest, via, ns)
	return nil
}
