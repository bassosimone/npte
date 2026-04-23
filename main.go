// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"

	"github.com/bassosimone/npte/internal/cli/container"
	"github.com/bassosimone/npte/internal/cli/doctor"
	"github.com/bassosimone/npte/internal/cli/gateway"
	"github.com/bassosimone/npte/internal/cli/netem"
	"github.com/bassosimone/npte/internal/cli/netns"
	"github.com/bassosimone/npte/internal/cli/star"
	"github.com/bassosimone/npte/internal/cli/tutorial"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	// Create dispatcher for `npte project`
	projectDisp := vclip.NewDispatcherCommand("npte project", vflag.ExitOnError)
	projectDisp.Exit = env.Exit
	projectDisp.Stderr = env.Stderr
	projectDisp.Stdout = env.Stdout
	projectDisp.AddDescription(
		"Manage npte projects. Each project is a directory under " + baseDir + "/ " +
			"containing network-namespace configuration and container filesystem trees.",
	)
	projectDisp.AddCommand("create", vclip.CommandFunc(projectCreateMain), "Create a new project.")

	// Create dispatcher for `npte netns`
	netnsDisp := vclip.NewDispatcherCommand("npte netns", vflag.ExitOnError)
	netnsDisp.Exit = env.Exit
	netnsDisp.Stderr = env.Stderr
	netnsDisp.Stdout = env.Stdout
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
	containerDisp.Exit = env.Exit
	containerDisp.Stderr = env.Stderr
	containerDisp.Stdout = env.Stdout
	containerDisp.AddDescription(
		"Manage lightweight containers backed by systemd-nspawn. "+
			"Each uses a debootstrap filesystem tree "+
			"and a network namespace.",
		"The tree directory is: "+baseDir+"/<project>/trees/<namepace>/.",
	)
	containerDisp.AddCommand("create", vclip.CommandFunc(containerCreateMain), "Bootstrap a container filesystem tree.")
	containerDisp.AddCommand("run", vclip.CommandFunc(containerRunMain), "Run a command inside a container.")

	// Create dispatcher for `npte netem`
	netemDisp := vclip.NewDispatcherCommand("npte netem", vflag.ExitOnError)
	netemDisp.Exit = env.Exit
	netemDisp.Stderr = env.Stderr
	netemDisp.Stdout = env.Stdout
	netemDisp.AddDescription(
		"Apply or clear traffic shaping on a client's access link. "+
			"Shapes both download and upload directions using tc/netem.",
		"For advanced usage (loss, batching, child qdiscs), use tc directly. "+
			"See 'npte tutorial netem' for the full reference.",
	)
	netemDisp.AddCommand("apply", vclip.CommandFunc(netemApplyMain), "Apply traffic shaping to a client.")
	netemDisp.AddCommand("clear", vclip.CommandFunc(netemClearMain), "Remove traffic shaping from a client.")

	// Create dispatcher for `npte`
	disp := vclip.NewDispatcherCommand("npte", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Network Performance Testing Environment (npte). "+
			"Test network client performance under realistic conditions using isolated "+
			"namespaces, traffic shaping, and optional lightweight containers.",
		"Creates a star topology of network namespaces around a central router. "+
			"Shape the client's access link with tc/netem to simulate real-world conditions. "+
			"Run commands directly in namespaces or inside systemd-nspawn containers.",
		"Run 'npte tutorial' for a complete walkthrough. "+
			"Namespaces are project-scoped; configuration is stored under "+baseDir+"/<project>/.",
	)
	disp.AddCommand("doctor", vclip.CommandFunc(doctor.Main), "Check for required external commands.")
	disp.AddCommand("tutorial_o", vclip.CommandFunc(tutorialMain), "Show the npte tutorials.")
	disp.AddCommand("tutorial", vclip.CommandFunc(tutorial.Main), "Show the npte tutorials.")
	disp.AddCommand("project", projectDisp, "Manage projects.")
	disp.AddCommand("netns_o", netnsDisp, "Manage network namespaces.")
	disp.AddCommand("netns", vclip.CommandFunc(netns.Main), "Manage network namespaces.")
	disp.AddCommand("gateway", vclip.CommandFunc(gateway.Main), "Manage namespaces as internet gateways.")
	disp.AddCommand("star", vclip.CommandFunc(star.Main), "Compose a fixed three-node star topology.")
	disp.AddCommand("container_o", containerDisp, "Manage lightweight containers.")
	disp.AddCommand("container", vclip.CommandFunc(container.Main), "Manage lightweight containers.")
	disp.AddCommand("netem_o", netemDisp, "Apply or clear traffic shaping.")
	disp.AddCommand("netem", vclip.CommandFunc(netem.Main), "Apply or clear traffic shaping.")

	// Run the root dispatcher
	vclip.Main(context.Background(), disp, os.Args[1:])
}
