// SPDX-License-Identifier: GPL-3.0-or-later

package netns

import (
	"context"
	"fmt"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/registry"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// listMain is the main of the `netns list` subcommand.
func listMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte netns list", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Lists the names of network namespaces managed by npte, one per line. "+
			"A namespace is managed when a marker file exists at "+
			"/run/npte/netns/<name>; markers are written by `npte netns create` "+
			"and removed by `npte netns destroy`.",
		"Outputs nothing (and exits 0) when no managed namespace is present, "+
			"so the result is safe to consume from a shell `for` loop.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args))

	unlock := registry.MustLock(ctx, env, false)
	defer unlock()

	names, err := registry.List(env)
	if err != nil {
		logx.Error("npte netns list: %s", err)
		env.Exit(2)
		return nil
	}
	for _, n := range names {
		fmt.Fprintln(env.Stdout, n)
	}
	return nil
}
