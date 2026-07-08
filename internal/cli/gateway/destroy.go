// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

import (
	"context"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// destroyMain is the main of the `gateway destroy` subcommand.
func destroyMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte gateway destroy", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Undoes `npte gateway create`: removes all host-side iptables rules "+
			"tagged \"npte:gw:<ns>\" via an iptables-save/restore pipeline, "+
			"and removes the host-side veth if it still exists.",
		"BUGS: the save/restore sweep rewrites every iptables table on the "+
			"host, which resets all packet/byte counters to zero, and any rule "+
			"that another actor (Docker, libvirt, ...) inserts during the brief "+
			"save-to-restore window is silently discarded — iptables-restore "+
			"applies each table atomically, but the sequence as a whole is an "+
			"unlocked read-modify-write. These side effects are accepted in "+
			"exchange for robustness: the tag-based sweep removes tagged rules "+
			"of any shape, regardless of which npte version created them. Avoid "+
			"running this while other software is reconfiguring the host firewall.",
		"BUGS: the SNAT rule that `npte gateway create` installs inside the "+
			"namespace is NOT removed; only host-side state is cleaned. This "+
			"follows from the intended lifecycle — create namespaces, layer a "+
			"gateway on top, destroy both — where the rule vanishes together "+
			"with the namespace (`npte netns destroy`). It only matters when "+
			"the namespace is kept and later re-made a gateway with a different "+
			"<subnet>: the stale rule still rewrites the source to the old "+
			"address, breaking uplink traffic. Recover with `sudo ip netns "+
			"exec <ns> iptables -t nat -F POSTROUTING` before re-creating.",
		"Safe to run in any order relative to `npte netns destroy`. If the "+
			"namespace was already destroyed, the host-side veth is gone (the "+
			"kernel cleans a veth when its far-end namespace disappears); the "+
			"veth-removal step tolerates that. If `npte gateway destroy` runs "+
			"first, `npte netns destroy` still works to clean up the namespace.",
		"Does NOT toggle net.ipv4.ip_forward back to 0. That setting is "+
			"host-wide and may be wanted by other things on the system (Docker, "+
			"other gateways, etc.); flipping it off would break them.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
	)
	usage.PositionalArgumentsUsage = "<ns>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	ns := fset.Args()[0]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte gateway destroy: %s", err)
		env.Exit(2)
		return nil
	}

	tag := "npte:gw:" + ns
	hostIface := "if-" + ns

	// Match the literal `--comment "<tag>"` token that modern
	// iptables-save (1.8+, both legacy and nft backends) emits for each
	// rule. Anchoring on the quoted comment argument ensures we only
	// delete rules whose comment is exactly our tag, not unrelated rules
	// whose comment happens to contain `npte:gw:<ns>` as a substring.
	// Pre-1.8 iptables-save sometimes omitted the quotes for shell-safe
	// values; we accept the resulting false negatives on those systems
	// (the operator can clean stale rules by hand) in exchange for the
	// substantially safer false-positive behavior on modern hosts.
	commentToken := `--comment "` + tag + `"`

	// Known, accepted side effects (see the BUGS paragraph in the help
	// text): the restore rewrites every table in the save output, so all
	// host-wide counters reset to zero, and rules inserted by another
	// actor during the save-to-restore window are lost. The sweep is
	// kept anyway because it deletes tagged rules of any shape,
	// independent of the npte version that created them; the symmetric
	// alternative (exact `iptables -D` mirrors of what create added)
	// breaks on version drift and duplicate rules.
	logx.Details("npte: remove host-side iptables rules tagged %q", tag)
	subprocess.MustPipeline(ctx, dryRun,
		[]string{"iptables-save"},
		// `--` terminates grep's option parsing so the pattern (which
		// itself starts with `--`) is not mistaken for a flag.
		[]string{"grep", "-Fv", "--", commentToken},
		[]string{"iptables-restore"},
	)

	// TODO(bassosimone): investigate also sweeping the namespace-side nat
	// table (tolerating an already-destroyed namespace) so that a namespace
	// kept alive across gateway re-creations does not retain a stale SNAT
	// rule pointing at the old subnet (see BUGS in the help text).

	logx.Details("npte: remove host-side veth %q (may already be gone)", hostIface)
	subprocess.MustRunTolerant(ctx, dryRun, "ip", "link", "del", hostIface)

	logx.Details("npte: gateway state for %q is cleaned up", ns)
	return nil
}
