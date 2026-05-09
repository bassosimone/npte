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

// resolvConf is the nameserver list installed in /etc/netns/<ns>/resolv.conf.
const resolvConf = "nameserver 1.1.1.1\nnameserver 8.8.8.8\n"

// createMain is the main of the `netns create` subcommand.
func createMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Creates a minimal network namespace: brings up loopback, sets endpoint-friendly "+
			"TCP buffer defaults, enables IPv4 forwarding (inert until the namespace has "+
			"two or more interfaces), and installs a namespace-scoped /etc/resolv.conf so "+
			"that processes launched via `ip netns exec` see its nameserver list.",
		"Loads the tcp_bbr kernel module on the host so that it is available as a "+
			"congestion-control choice inside the namespace.",
		"The namespace name must match ^[a-z][a-z0-9]*$ and is capped at 12 characters "+
			"so that peer-facing interfaces named \"if-<peer>\" (see `npte netns connect`) "+
			"fit in IFNAMSIZ (15 bytes).",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
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
		logx.Error("npte netns create: %s", err)
		env.Exit(2)
		return nil
	}

	unlock := registry.MustLock(ctx, env, dryRun)
	defer unlock()

	logx.Details("npte: load tcp_bbr kernel module")
	subprocess.MustRun(ctx, dryRun, "modprobe", "tcp_bbr")

	logx.Details("npte: create namespace %q", ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "add", ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns, "ip", "link", "set", "lo", "up")

	logx.Details("npte: configure sysctls in %q", ns)
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns,
		"sysctl", "-w", "net.ipv4.tcp_rmem=4096 131072 33554432")
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns,
		"sysctl", "-w", "net.ipv4.tcp_wmem=4096 131072 33554432")
	subprocess.MustRun(ctx, dryRun, "ip", "netns", "exec", ns,
		"sysctl", "-w", "net.ipv4.ip_forward=1")

	// Workaround for https://github.com/uutils/coreutils/issues/11532
	logx.Details("ensure /etc/netns/%s is clean (workaround for uutils/coreutils#11532)", ns)
	subprocess.MustRun(ctx, dryRun, "rm", "-rf", "/etc/netns/"+ns)

	logx.Details("npte: install resolv.conf at /etc/netns/%s/resolv.conf", ns)
	subprocess.MustPipeTo(ctx, dryRun, []byte(resolvConf),
		"install", "-D", "-m", "0644", "/dev/stdin",
		"/etc/netns/"+ns+"/resolv.conf")

	logx.Details("npte: register %q in the npte registry", ns)
	registry.MustRegister(ctx, dryRun, ns)

	logx.Details("npte: namespace %q is ready", ns)
	return nil
}
