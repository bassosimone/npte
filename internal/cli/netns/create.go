// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

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
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	return nil
}
