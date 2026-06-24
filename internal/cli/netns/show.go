// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"fmt"
	"slices"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/registry"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// showMain is the main of the `netns show` subcommand.
func showMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns show", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Inspects a network namespace managed by npte. Runs a fixed set of "+
			"diagnostic commands and emits their output on stdout, each "+
			"preceded by a `=== <section> ===` header.",
		"The sections are: link (interfaces, MAC, MTU, up/down), addr "+
			"(IPv4/IPv6 addresses), route and route6 (per-family routing "+
			"tables), qdisc (traffic shaping with packet/drop counters via "+
			"`tc -s`), neigh (ARP/NDP table), sockets (listening TCP/UDP "+
			"sockets with owning processes via `ss -tunlp`), and pids "+
			"(PIDs of processes whose net namespace is this one, one per "+
			"line, in the order `ip netns pids` returns them — kernel "+
			"readdir order, not numeric).",
		"Use --section to restrict the dump to a subset of sections (e.g. "+
			"`--section route --section qdisc`). When --section is not "+
			"given, all sections are emitted. Names not in the canonical "+
			"set above are silently ignored, so it is safe to pass through "+
			"an arbitrary list. Output order follows the canonical section "+
			"order regardless of flag order, so dumps are stable across "+
			"invocations.",
		"Refuses to operate on a namespace that npte does not manage. "+
			"Per-section commands are tolerant of non-zero exit (e.g. an "+
			"empty table prints empty output rather than aborting the run).",
	)
	usage.PositionalArgumentsUsage = "<ns>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var wantSections []string
	fset.StringSliceVar(&wantSections, 0, "section",
		"Emit only the named `section` (repeatable). Valid names: "+
			"link, addr, route, route6, qdisc, neigh, sockets. Unknown "+
			"names are silently ignored.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	// NOPASSWD audit invariant: this command is part of the set that
	// `npte sudoers` allowlists for sudo execution without a password
	// (see CLAUDE.md in this package). Every flag value, positional, or
	// environment value forwarded to a subprocess must be validated
	// here — fail loud, prefer hardcoded literals, never trust the
	// caller's bytes. A missing check is a passwordless privesc hole.

	ns := fset.Args()[0]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netns show: %s", err)
		env.Exit(2)
		return nil
	}

	unlock := registry.MustLock(ctx, env, false)
	defer unlock()

	if err := registry.RequireManaged(env, false, ns); err != nil {
		logx.Error("npte netns show: %s", err)
		env.Exit(2)
		return nil
	}

	// Each section runs one read-only diagnostic command inside the
	// namespace. `ip -n <ns> ...` covers the ip-native subcommands; tc
	// and ss are not `ip` subcommands, so they go through `ip netns
	// exec`. RunTolerant is used so that a non-zero exit on one
	// section (e.g. ss missing on a stripped-down system) does not
	// abort the rest of the dump.
	sections := []struct {
		title string
		argv  []string
	}{
		{"link", []string{"-n", ns, "link", "show"}},
		{"addr", []string{"-n", ns, "addr", "show"}},
		{"route", []string{"-n", ns, "route", "show"}},
		{"route6", []string{"-n", ns, "-6", "route", "show"}},
		{"qdisc", []string{"netns", "exec", ns, "tc", "-s", "qdisc", "show"}},
		{"neigh", []string{"-n", ns, "neigh", "show"}},
		{"sockets", []string{"netns", "exec", ns, "ss", "-tunlp"}},
		{"pids", []string{"netns", "pids", ns}},
	}
	for _, s := range sections {
		if len(wantSections) > 0 && !slices.Contains(wantSections, s.title) {
			continue
		}
		fmt.Fprintf(env.Stdout, "=== %s ===\n", s.title)
		subprocess.MustRunTolerant(ctx, false, "ip", s.argv...)
		fmt.Fprintln(env.Stdout)
	}
	return nil
}
