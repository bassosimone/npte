// SPDX-License-Identifier: GPL-3.0-or-later

package gateway

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

// createMain is the main of the `gateway create` subcommand.
func createMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte gateway create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Turns an existing namespace into an internet gateway. Wires an uplink veth "+
			"pair (\"if-<ns>\" on the host, \"if-host\" inside the namespace), addresses "+
			"both ends from <subnet> (.1 on the host, .2 inside the namespace), installs "+
			"a default route inside the namespace via the host side, enables IPv4 "+
			"forwarding on the host, and installs host-side MASQUERADE plus permissive "+
			"FORWARD rules scoped to <subnet> so traffic egresses through <ext-iface>.",
		"Additionally installs SNAT inside the namespace on the uplink interface. "+
			"For a singleton gateway this is a no-op; for a transit gateway (nested "+
			"namespaces behind this one) it hides internal source addresses from the "+
			"host so the host does not need routes for internal subnets.",
		"All host-side iptables rules carry the comment \"npte:gw:<ns>\" so that "+
			"`npte gateway destroy` can find and delete them without having to "+
			"re-derive the original arguments.",
		"The namespace must already exist (see `npte netns create`). <subnet> must be "+
			"an IPv4 prefix with room for at least .1 and .2 (i.e. /30 or wider); host "+
			"bits in the argument are ignored. The command is not idempotent: re-running "+
			"it will fail loud on the first conflicting step. To recover, run `npte "+
			"gateway destroy` first.",
		"BUGS: the veth pair is created in the host namespace and one end is then "+
			"moved into <ns>; the two steps are not atomic. If <ns> does not exist "+
			"(or the move fails for any other reason), the fully-formed pair is left "+
			"behind on the host, and the stale fixed-name \"if-host\" end makes every "+
			"subsequent `npte gateway create` fail with EEXIST. Recover with "+
			"`sudo ip link del if-host` (deleting either end removes the whole pair).",
		"BUGS: the host-side \"if-<ns>\" uplink persists for the gateway's lifetime "+
			"and occupies the name that `npte netns connect` transiently needs on the "+
			"host whenever one of the peers is named <ns>, so such connects fail with "+
			"EEXIST. Wire all `npte netns connect` links involving <ns> before running "+
			"this command.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
	)
	usage.PositionalArgumentsUsage = "<ns> <subnet> <ext-iface>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 3
	fset.MaxPositionalArgs = 3
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	ns := fset.Args()[0]
	subnet := fset.Args()[1]
	extIface := fset.Args()[2]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte gateway create: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.CIDR(subnet); err != nil {
		logx.Error("npte gateway create: %s", err)
		env.Exit(2)
		return nil
	}
	if err := validate.IfaceName(extIface); err != nil {
		logx.Error("npte gateway create: %s", err)
		env.Exit(2)
		return nil
	}

	// Canonicalize the subnet (strip host bits) and derive the two host
	// addresses. Require IPv4 and a prefix length that leaves room for .1
	// and .2 as distinct, in-prefix host addresses.
	prefix := netip.MustParsePrefix(subnet).Masked()
	if !prefix.Addr().Is4() {
		logx.Error("npte gateway create: subnet %q is not IPv4", subnet)
		env.Exit(2)
		return nil
	}
	hostAddr := prefix.Addr().Next()
	nsAddr := hostAddr.Next()
	if !prefix.Contains(hostAddr) || !prefix.Contains(nsAddr) || hostAddr == nsAddr {
		logx.Error("npte gateway create: subnet %q is too small for .1/.2 host addresses", subnet)
		env.Exit(2)
		return nil
	}

	canonicalSubnet := prefix.String()
	plen := prefix.Bits()
	hostCIDR := netip.PrefixFrom(hostAddr, plen).String()
	nsCIDR := netip.PrefixFrom(nsAddr, plen).String()
	// TODO(bassosimone): a namespace named "host" passes validate.NetnsName
	// but makes hostIface == nsIface == "if-host", so the `ip link add`
	// below fails with a raw kernel error instead of a validation message.
	// Consider rejecting the degenerate name "host" (here or in validate).
	hostIface := "if-" + ns
	nsIface := "if-host"
	tag := "npte:gw:" + ns
	runtimex.PanicOnError0(validate.IfaceName(hostIface))
	runtimex.PanicOnError0(validate.IfaceName(nsIface))

	// Create-then-move is not atomic: if the move fails, the fully-formed
	// pair stays behind on the host and the fixed-name "if-host" end blocks
	// every subsequent `gateway create` (see BUGS in the help text).
	//
	// TODO(bassosimone): investigate replacing the two steps with the single
	// atomic form `ip link add <hostIface> type veth peer name <nsIface>
	// netns <ns>`, which creates the device in the current namespace and the
	// peer directly inside <ns>: if <ns> does not exist, nothing is created.
	// Beware the near-miss variant that puts `netns` on the *device* instead
	// of the peer: when the peer carries no netns attribute of its own it
	// inherits the device's target namespace, so BOTH ends land inside <ns>.
	// Requires a live root smoke test (and updating the pinned dry-run
	// scripts in the tests) before switching.
	logx.Details("npte: create uplink veth pair %q <-> %q", hostIface, nsIface)
	subprocess.MustRun(ctx, dryRun, "ip", "link", "add", hostIface, "type", "veth", "peer", "name", nsIface)

	logx.Details("npte: move %q into namespace %q", nsIface, ns)
	subprocess.MustRun(ctx, dryRun, "ip", "link", "set", nsIface, "netns", ns)

	logx.Details("npte: assign %s to %q on host", hostCIDR, hostIface)
	subprocess.MustRun(ctx, dryRun, "ip", "addr", "add", hostCIDR, "dev", hostIface)

	logx.Details("npte: assign %s to %q inside %q", nsCIDR, nsIface, ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns, "ip", "addr", "add", nsCIDR, "dev", nsIface)

	logx.Details("npte: bring %q up on host", hostIface)
	subprocess.MustRun(ctx, dryRun, "ip", "link", "set", hostIface, "up")

	logx.Details("npte: bring %q up inside %q", nsIface, ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns, "ip", "link", "set", nsIface, "up")

	logx.Details("npte: add default route via %s inside %q", hostAddr, ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns, "ip", "route", "add", "default", "via", hostAddr.String())

	logx.Details("npte: enable IPv4 forwarding on host")
	subprocess.MustRun(ctx, dryRun, "sysctl", "-w", "net.ipv4.ip_forward=1")

	logx.Details("npte: install host MASQUERADE for %s out of %q", canonicalSubnet, extIface)
	subprocess.MustRun(ctx, dryRun, "iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", canonicalSubnet, "-o", extIface,
		"-m", "comment", "--comment", tag,
		"-j", "MASQUERADE")

	logx.Details("npte: install host FORWARD rules for %s", canonicalSubnet)
	subprocess.MustRun(ctx, dryRun, "iptables", "-I", "FORWARD",
		"-s", canonicalSubnet,
		"-m", "comment", "--comment", tag,
		"-j", "ACCEPT")
	subprocess.MustRun(ctx, dryRun, "iptables", "-I", "FORWARD",
		"-d", canonicalSubnet,
		"-m", "comment", "--comment", tag,
		"-j", "ACCEPT")

	logx.Details("npte: install uplink SNAT inside %q", ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns,
		"iptables", "-t", "nat", "-A", "POSTROUTING",
		"-o", nsIface,
		"-m", "comment", "--comment", tag,
		"-j", "SNAT", "--to-source", nsAddr.String())

	logx.Details("npte: namespace %q is a gateway for %s via %q", ns, canonicalSubnet, extIface)
	return nil
}
