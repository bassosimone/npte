// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"math"
	"os"
	"strings"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// runMain is the main of the `netns run` subcommand.
//
// Execution strategy: `ip netns exec <ns> runuser -u <user> -- env K=V... <cmd> [args...]`.
//
// Why this pipeline rather than `systemd-run --property=NetworkNamespacePath=...`:
//
//  1. The `/etc/netns/<ns>/*` → `/etc/*` overlay (which gives the namespace
//     its own resolv.conf, installed by `netns create`) is not a property
//     of the network namespace. It is implemented in userspace by
//     `ip netns exec` itself: setns, unshare(CLONE_NEWNS), rslave /, then
//     bind-mount each file under `/etc/netns/<ns>/` over the corresponding
//     `/etc/<file>`. Any tool that enters the netns by a different path
//     (systemd-run via NetworkNamespacePath, nsenter, direct setns) gets
//     none of this and sees the host's `/etc/resolv.conf`. Going through
//     `ip netns exec` is the only way to inherit the overlay for free.
//
//  2. `ip netns exec` spawns the target as an honest child of the invoking
//     shell: it keeps the controlling TTY, job control works (^C, ^Z,
//     jobs/fg/bg), `ps` under the caller's tty shows it, SIGHUP flows from
//     the terminal, and `wait(2)` returns the real exit status.
//     `systemd-run` routes through PID 1 via D-Bus; the command then runs
//     as a transient unit under init, with stdio proxied and signals
//     relayed. That is the wrong shape for an interactive "step into the
//     namespace and poke at it" command, which is the primary use case.
//
// `runuser` (util-linux) drops privilege from root to <user> without PAM
// prompts or sudoers config; `env` (coreutils) injects the requested
// KEY=VALUE pairs after the privilege drop. Both are universally available
// on target platforms.
//
// An alternative worth naming: `systemd-run --pipe --quiet --collect
// --property=NetworkNamespacePath=/run/netns/<ns>
// --property=BindPaths=/etc/netns/<ns>/resolv.conf:/etc/resolv.conf
// --uid=<user> --setenv=K=V -- <cmd>`. This form is preferable when the
// workload is long-running and benefits from a transient cgroup-scoped
// unit (per-run CPU/memory/IO accounting, `systemctl status`,
// `journalctl -u`, detachment from the invoking shell, `MemoryMax=` /
// `CPUQuota=` per invocation, atomic kill of the whole unit). It is a
// poor fit for interactive work because it gives up the shell-child
// semantics above. If that workload ever needs a first-class home, it
// belongs in a separate subcommand that advertises the detachment and
// the unit name rather than hiding a D-Bus call inside `run`.
func runMain(ctx context.Context, args []string) error {
	env := testable.Env

	userFlag := os.Getenv("SUDO_USER")
	var envFlags []string
	var dryRun bool

	fset := vflag.NewFlagSet("npte netns run", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Runs a command inside a network namespace. The command is an honest "+
			"child of the invoking shell: it keeps the controlling TTY and job "+
			"control works as expected.",
		"Entry is via `ip netns exec`, which also bind-mounts every file under "+
			"/etc/netns/<ns>/ over the corresponding /etc/ path, so the namespace-"+
			"scoped resolv.conf installed by `npte netns create` is in effect.",
		"By default the command runs as $SUDO_USER; use -u/--user to override "+
			"(e.g. --user root for privileged operations). Use -e/--env to inject "+
			"KEY=VALUE pairs; the inner process otherwise starts with runuser(1)'s "+
			"default environment for the target user.",
		"With --dry-run, prints a round-trippable shell line to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.PositionalArgumentsUsage = "<ns> <command> [args...]"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.StringVar(&userFlag, 'u', "user", "Run as `USER` (default: $SUDO_USER).")
	fset.StringSliceVar(&envFlags, 'e', "env", "Set environment variable `KEY=VALUE` (repeatable).")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	ns := fset.Args()[0]
	if err := validate.NetnsName(ns); err != nil {
		logx.Error("npte netns run: %s", err)
		env.Exit(2)
		return nil
	}
	if userFlag == "" {
		logx.Error("npte netns run: no user specified; " +
			"set $SUDO_USER (e.g. run via sudo) or pass -u/--user explicitly")
		env.Exit(2)
		return nil
	}
	if err := validate.Username(userFlag); err != nil {
		logx.Error("npte netns run: %s", err)
		env.Exit(2)
		return nil
	}
	for _, ev := range envFlags {
		key, _, ok := strings.Cut(ev, "=")
		if !ok {
			logx.Error("npte netns run: -e value must be KEY=VALUE, got %q", ev)
			env.Exit(2)
			return nil
		}
		if err := validate.EnvVarName(key); err != nil {
			logx.Error("npte netns run: -e: %s", err)
			env.Exit(2)
			return nil
		}
	}

	// Assemble: ip netns exec <ns> runuser -u <user> -- env K=V... <cmd> [args...]
	argv := []string{"netns", "exec", ns, "runuser", "-u", userFlag, "--", "env"}
	argv = append(argv, envFlags...)
	argv = append(argv, fset.Args()[1:]...)

	logx.Details("npte: enter namespace %q as user %q", ns, userFlag)
	subprocess.MustRun(ctx, dryRun, "ip", argv...)
	return nil
}
