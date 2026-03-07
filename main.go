// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"

	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	// Create dispatcher for `npte project`
	projectDisp := vclip.NewDispatcherCommand("npte project", vflag.ExitOnError)
	projectDisp.AddDescription(
		"Manage npte projects. Each project is a directory under "+baseDir+"/ "+
			"containing network configuration and container filesystem trees.",
		"All subcommands must be run as root (e.g., via sudo).",
	)
	projectDisp.AddCommand("create", vclip.CommandFunc(projectCreateMain), "Create a new project.")

	// Create dispatcher for `npte netns`
	netnsDisp := vclip.NewDispatcherCommand("npte netns", vflag.ExitOnError)
	netnsDisp.AddDescription(
		"Manage network namespaces arranged in a star topology around a central router. "+
			"The router namespace is the only one with internet access via host NAT.",
		"All subcommands must be run as root (e.g., via sudo).",
	)
	netnsDisp.AddCommand("create", vclip.CommandFunc(netnsCreateMain), "Add a namespace to the configuration.")
	netnsDisp.AddCommand("up", vclip.CommandFunc(netnsUpMain), "Bring up the network from configuration.")
	netnsDisp.AddCommand("down", vclip.CommandFunc(netnsDownMain), "Tear down the network.")
	netnsDisp.AddCommand("run", vclip.CommandFunc(netnsRunMain), "Run a command inside a namespace.")

	// Create dispatcher for `npte container`
	containerDisp := vclip.NewDispatcherCommand("npte container", vflag.ExitOnError)
	containerDisp.AddDescription(
		"Manage lightweight containers backed by systemd-nspawn. "+
			"Each container is a debootstrap filesystem tree bound to a network namespace.",
		"All subcommands must be run as root (e.g., via sudo).",
	)
	containerDisp.AddCommand("create", vclip.CommandFunc(containerCreateMain), "Bootstrap a container filesystem tree.")
	containerDisp.AddCommand("run", vclip.CommandFunc(containerRunMain), "Run a command inside a container.")

	// Create dispatcher for `npte`
	disp := vclip.NewDispatcherCommand("npte", vflag.ExitOnError)
	disp.AddDescription(
		"Network Performance Testing Environment (npte).",
		"Creates isolated network namespaces connected through a central router, "+
			"with the ability to bind the namespaces to lightweight containers run using systemd-nspawn(1).",
		"Namespaces are project-scoped. Per-project configuration is stored under "+baseDir+"/<project>/.",
	)
	disp.AddCommand("project", projectDisp, "Manage projects.")
	disp.AddCommand("netns", netnsDisp, "Manage network namespaces.")
	disp.AddCommand("container", containerDisp, "Manage lightweight containers.")

	// Run the root dispatcher
	vclip.Main(context.Background(), disp, os.Args[1:])
}
