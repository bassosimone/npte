// SPDX-License-Identifier: GPL-3.0-or-later

package netns

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

// connectMain is the main of the `netns connect` subcommand.
func connectMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns connect", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Wires two existing network namespaces with a veth pair. Inside each "+
			"namespace the interface is named \"if-<peer>\": inside <left> the "+
			"interface toward <right> is \"if-<right>\", and inside <right> the "+
			"interface toward <left> is \"if-<left>\". This makes `ip link show` "+
			"inside each namespace self-describing.",
		"Both namespaces must already exist (see `npte netns create`). Addressing "+
			"is a separate concern; this command brings the two ends up with no "+
			"L3 configuration.",
		"BUGS: the veth pair is created in the host namespace and the two ends are "+
			"then moved; the steps are not atomic. If a move fails (e.g. a namespace "+
			"disappeared out-of-band after the registry check), the leftover ends stay "+
			"on the host and block reconnecting the same pair with EEXIST. Recover with "+
			"`sudo ip link del if-<right>` (deleting either end removes the whole pair).",
		"BUGS: for the same reason, the transient host-side names can collide with "+
			"interfaces already present on the host. In particular, `npte gateway "+
			"create <gw> ...` leaves a permanent \"if-<gw>\" uplink interface on the "+
			"host, so a later connect involving <gw> fails with EEXIST. Wire all "+
			"connections for a namespace before turning it into a gateway.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
	)
	usage.PositionalArgumentsUsage = "<left> <right>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	// NOPASSWD audit invariant: this command is part of the set that
	// `npte sudoers` allowlists for sudo execution without a password
	// (see CLAUDE.md in this package). Every flag value, positional, or
	// environment value forwarded to a subprocess must be validated
	// here — fail loud, prefer hardcoded literals, never trust the
	// caller's bytes. A missing check is a passwordless privesc hole.

	left := fset.Args()[0]
	right := fset.Args()[1]
	if err := validate.NetnsName(left); err != nil {
		logx.Error("npte netns connect: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.NetnsName(right); err != nil {
		logx.Error("npte netns connect: %s", err)
		env.Exit(2)
		return nil
	}
	if left == right {
		logx.Error("npte netns connect: left and right must differ (both are %q)", left)
		env.Exit(2)
		return nil
	}

	unlock := registry.MustLock(ctx, env, dryRun)
	defer unlock()

	if err := registry.RequireManaged(env, dryRun, left); err != nil {
		logx.Error("npte netns connect: %s", err)
		env.Exit(2)
		return nil
	}
	if err := registry.RequireManaged(env, dryRun, right); err != nil {
		logx.Error("npte netns connect: %s", err)
		env.Exit(2)
		return nil
	}

	// TODO(bassosimone): a namespace named "host" passes validate.NetnsName
	// but yields an "if-host" end here, which transiently lives in the host
	// namespace and can collide with a concurrent `gateway create` (whose
	// pair also transiently owns "if-host"). Consider rejecting the
	// degenerate name "host" (see the sibling TODO in gateway/create.go).
	leftIf := "if-" + right
	rightIf := "if-" + left
	runtimex.PanicOnError0(validate.IfaceName(leftIf))
	runtimex.PanicOnError0(validate.IfaceName(rightIf))

	// Create-then-move is not atomic: a failed move leaves the pair behind
	// on the host (see BUGS in the help text).
	//
	// TODO(bassosimone): investigate creating both ends in place atomically
	// (`ip link add <leftIf> netns <left> type veth peer name <rightIf>
	// netns <right>`). See the fuller TODO in gateway/create.go — including
	// the gotcha that a peer without its own netns attribute inherits the
	// device's target namespace — before changing either site. Atomic
	// creation would also remove the host-namespace name collision with
	// `gateway create`'s persistent "if-<ns>" uplink (see BUGS).
	logx.Details("npte: create veth pair %q <-> %q", leftIf, rightIf)
	subprocess.MustRun(ctx, dryRun, "ip", "link", "add", leftIf, "type", "veth", "peer", "name", rightIf)

	logx.Details("npte: move %q into namespace %q", leftIf, left)
	subprocess.MustRun(ctx, dryRun, "ip", "link", "set", leftIf, "netns", left)

	logx.Details("npte: move %q into namespace %q", rightIf, right)
	subprocess.MustRun(ctx, dryRun, "ip", "link", "set", rightIf, "netns", right)

	logx.Details("npte: bring %q up inside %q", leftIf, left)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", left, "ip", "link", "set", leftIf, "up")

	logx.Details("npte: bring %q up inside %q", rightIf, right)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", right, "ip", "link", "set", rightIf, "up")

	logx.Details("npte: veth pair %q @ %s <-> %q @ %s is ready", leftIf, left, rightIf, right)
	return nil
}
