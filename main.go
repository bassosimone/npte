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
		"Manage npte projects. Each project is a directory under " + baseDir + "/ " +
			"containing network-namespace configuration and container filesystem trees.",
	)
	projectDisp.AddCommand("create", vclip.CommandFunc(projectCreateMain), "Create a new project.")

	// Create dispatcher for `npte netns`
	netnsDisp := vclip.NewDispatcherCommand("npte netns", vflag.ExitOnError)
	netnsDisp.AddDescription(
		"Manage per-project network namespaces arranged in a star topology around a central router. "+
			"The router namespace is the only one with internet access via host through NAT.",
		"The configuration lives at:",
		"    "+baseDir+"/<project>/config/netns.json",
	)
	netnsDisp.AddCommand("create", vclip.CommandFunc(netnsCreateMain), "Add a namespace to the configuration.")
	netnsDisp.AddCommand("up", vclip.CommandFunc(netnsUpMain), "Bring up the configured namespaces.")
	netnsDisp.AddCommand("down", vclip.CommandFunc(netnsDownMain), "Tear down the namespaces.")
	netnsDisp.AddCommand("run", vclip.CommandFunc(netnsRunMain), "Run a command inside a namespace.")
	netnsDisp.AddCommand("show", vclip.CommandFunc(netnsShowMain), "Show the network topology.")
	netnsDisp.AddCommand("status", vclip.CommandFunc(netnsStatusMain), "Check whether the namespaces are up.")

	// Create dispatcher for `npte container`
	containerDisp := vclip.NewDispatcherCommand("npte container", vflag.ExitOnError)
	containerDisp.AddDescription(
		"Manage lightweight containers backed by systemd-nspawn. "+
			"Each uses a debootstrap filesystem tree "+
			"and a network namespace.",
		"The tree directory is: "+baseDir+"/<project>/trees/<namepace>/.",
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
	disp.AddCommand("doctor", vclip.CommandFunc(doctorMain), "Check for required external commands.")
	disp.AddCommand("tutorial", vclip.CommandFunc(tutorialMain), "Show the npte tutorials.")
	disp.AddCommand("project", projectDisp, "Manage projects.")
	disp.AddCommand("netns", netnsDisp, "Manage network namespaces.")
	disp.AddCommand("container", containerDisp, "Manage lightweight containers.")

	// Run the root dispatcher
	vclip.Main(context.Background(), disp, os.Args[1:])
}
