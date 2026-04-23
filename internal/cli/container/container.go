// SPDX-License-Identifier: GPL-3.0-or-later

// Package container implements the container subcommand.
package container

import (
	"context"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

// Main is the main of the container subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte container", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Manage lightweight containers as composable primitives. Each subcommand "+
			"performs one operation: bootstrap a filesystem tree with debootstrap, run "+
			"a one-shot command inside a tree with systemd-nspawn, or boot a tree as "+
			"a systemd machine.",
		"The filesystem tree and the network namespace are orthogonal inputs: the "+
			"tree is any directory path, the namespace (optional) is any name under "+
			"/run/netns/. Nothing is registered or persisted; topologies are built "+
			"imperatively by composing with `npte netns` and `npte gateway`. Requires root.",
	)
	disp.AddCommand("create", vclip.CommandFunc(createMain), "Bootstrap a filesystem tree with debootstrap.")
	disp.AddCommand("run", vclip.CommandFunc(runMain), "Run a command inside a tree with systemd-nspawn.")
	disp.AddCommand("boot", vclip.CommandFunc(bootMain), "Boot a tree as a systemd-nspawn machine.")

	return disp.Main(ctx, args)
}
