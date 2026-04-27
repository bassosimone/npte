// SPDX-License-Identifier: GPL-3.0-or-later

package netem

import (
	"context"
	"strings"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/registry"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// applyMain is the main of the `netem apply` subcommand.
func applyMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netem apply", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Install `root handle 1: netem` on <if> inside <ns> with the knobs "+
			"provided via flags. Flag values are passed verbatim to tc; see "+
			"`man tc-netem` for value grammar.",
		"Examples of value grammar: --delay \"10ms 2ms distribution paretonormal\", "+
			"--loss \"gemodel 0.1 0.05 0.9 0.95\", --rate \"10mbit 1000 500\", "+
			"--slot \"5ms 10ms packets 64\".",
		"Shaping is one-directional: the qdisc affects packets egressing <if> "+
			"inside <ns>. For an asymmetric link, run this command twice on "+
			"the two ns+iface endpoints of the veth pair.",
		"With --child <kind>, a child qdisc is attached at parent 1: handle 2: "+
			"for AQM experiments (FIFO behaviour is modelled via netem's own "+
			"--limit instead). The kind is one of: "+
			strings.Join(validate.AllowedChildQdiscs, ", ")+". Per-kind knobs "+
			"are exposed as separate flags (e.g. --cake-bandwidth) added on "+
			"demand; reach for `sudo ip netns exec <ns> tc ...` directly if "+
			"you need a knob this command does not surface.",
		"The command is not idempotent: re-running it on an already-shaped "+
			"interface will fail loud. Run `npte netem clear <ns> <if>` first.",
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
	var delay, loss, limit, rate, slot, child, cakeBandwidth string
	fset.StringVar(&delay, 0, "delay", "Pass-through to netem `delay` (e.g. \"10ms\", \"10ms 2ms distribution paretonormal\").")
	fset.StringVar(&loss, 0, "loss", "Pass-through to netem `loss` (e.g. \"1%\", \"gemodel 0.1 0.05 0.9 0.95\").")
	fset.StringVar(&limit, 0, "limit", "Pass-through to netem `limit` in packets (e.g. \"1000\").")
	fset.StringVar(&rate, 0, "rate", "Pass-through to netem `rate` (e.g. \"10mbit\", \"10mbit 1000 500\").")
	fset.StringVar(&slot, 0, "slot", "Pass-through to netem `slot` (e.g. \"5ms 10ms packets 64\").")
	fset.StringVar(&child, 0, "child",
		"Child qdisc kind attached at parent 1: (one of: "+
			strings.Join(validate.AllowedChildQdiscs, ", ")+").")
	fset.StringVar(&cakeBandwidth, 0, "cake-bandwidth", "Bandwidth for `cake` (e.g. \"30mbit\"); requires --child cake.")
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
		logx.Error("npte netem apply: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.IfaceName(iface); err != nil {
		logx.Error("npte netem apply: %s", err)
		env.Exit(2)
		return nil
	}

	// Build the netem arg list in canonical order (matches `man tc-netem`).
	// Netem is order-insensitive between knobs, but consistent emission makes
	// dry-run output easier to diff across scenarios.
	//
	// Each flag is validated against a per-flag whole-value regex modeled
	// on the documented grammar (see internal/validate/netem.go). On
	// success we mechanically split with strings.Fields — no atom in any
	// netem grammar contains whitespace, so quoting (shellquote.Split) is
	// not needed here.
	netemArgs := []string{}
	for _, kv := range []struct {
		key, value string
		validate   func(string) error
	}{
		{"delay", delay, validate.NetemDelay},
		{"loss", loss, validate.NetemLoss},
		{"limit", limit, validate.NetemLimit},
		{"rate", rate, validate.NetemRate},
		{"slot", slot, validate.NetemSlot},
	} {
		if kv.value == "" {
			continue
		}
		if err := kv.validate(kv.value); err != nil {
			logx.Error("npte netem apply: --%s: %s", kv.key, err)
			env.Exit(2)
			return nil
		}
		netemArgs = append(netemArgs, kv.key)
		netemArgs = append(netemArgs, strings.Fields(kv.value)...)
	}
	if len(netemArgs) <= 0 && child == "" {
		logx.Error("npte netem apply: at least one of --delay/--loss/--limit/--rate/--slot/--child must be provided")
		env.Exit(2)
		return nil
	}

	unlock := registry.MustLock(ctx, env, dryRun)
	defer unlock()

	if err := registry.RequireManaged(env, ns); err != nil {
		logx.Error("npte netem apply: %s", err)
		env.Exit(2)
		return nil
	}

	rootArgs := append(
		[]string{"ip", "netns", "exec", ns, "tc", "qdisc", "add", "dev", iface, "root", "handle", "1:", "netem"},
		netemArgs...,
	)
	logx.Details("npte: install root netem qdisc on %q inside %q", iface, ns)
	subprocess.MustRun(ctx, dryRun, rootArgs[0], rootArgs[1:]...)

	if child != "" {
		if err := validate.ChildQdiscKind(child); err != nil {
			logx.Error("npte netem apply: --child: %s", err)
			env.Exit(2)
			return nil
		}
		childArgs := []string{
			"ip", "netns", "exec", ns,
			"tc", "qdisc", "add", "dev", iface,
			"parent", "1:", "handle", "2:", child,
		}
		// Each case validates and consumes its own knob values, so a
		// value whose owning --child kind isn't selected stays unused
		// (no validation, no append). Add new per-kind knobs by
		// extending the matching case.
		switch child {
		case "cake":
			if cakeBandwidth != "" {
				if err := validate.NetemRate(cakeBandwidth); err != nil {
					logx.Error("npte netem apply: --cake-bandwidth: %s", err)
					env.Exit(2)
					return nil
				}
				childArgs = append(childArgs, "bandwidth", cakeBandwidth)
			}
		}
		logx.Details("npte: attach child qdisc %q on %q inside %q", child, iface, ns)
		subprocess.MustRun(ctx, dryRun, childArgs[0], childArgs[1:]...)
	}

	logx.Details("npte: shaping installed on %q inside %q", iface, ns)
	return nil
}
