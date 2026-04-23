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
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
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
	runtimex.PanicOnError0(fset.Parse(args))

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
	hostIface := "if-" + ns
	nsIface := "if-host"
	tag := "npte:gw:" + ns
	runtimex.PanicOnError0(validate.IfaceName(hostIface))
	runtimex.PanicOnError0(validate.IfaceName(nsIface))

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
