// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"

	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	// Create dispatcher for `npte net`
	netDisp := vclip.NewDispatcherCommand("npte net", vflag.ExitOnError)
	netDisp.AddDescription(
		"Manage network namespaces arranged in a star topology around a central router. "+
			"The router namespace is the only one with internet access via host NAT.",
	)
	netDisp.AddCommand("init", vclip.CommandFunc(netInitMain), "Create the router and network infrastructure.")
	netDisp.AddCommand("create", vclip.CommandFunc(netCreateMain), "Create a network endpoint.")
	netDisp.AddCommand("run", vclip.CommandFunc(netRunMain), "Run a command inside a namespace.")
	netDisp.AddCommand("destroy", vclip.CommandFunc(netDestroyMain), "Destroy all network namespaces.")

	// Create dispatcher for `npte container`
	containerDisp := vclip.NewDispatcherCommand("npte container", vflag.ExitOnError)
	containerDisp.AddDescription(
		"Manage lightweight containers backed by systemd-nspawn. "+
			"Each container is a debootstrap filesystem tree bound to a network namespace.",
	)
	containerDisp.AddCommand("create", vclip.CommandFunc(containerCreateMain), "Create a filesystem tree.")
	containerDisp.AddCommand("enter", vclip.CommandFunc(containerEnterMain), "Enter a filesystem tree.")

	// Create dispatcher for `npte`
	disp := vclip.NewDispatcherCommand("npte", vflag.ExitOnError)
	disp.AddDescription(
		"Network Performance Testing Environment (npte).",
		"Creates isolated network namespaces connected through a central router, with optional "+
			"netem shaping and the ability to bind the namespaces to lightweight containers via systemd-nspawn.",
	)
	disp.AddCommand("init", vclip.CommandFunc(initMain), "Initialize the .npte directory.")
	disp.AddCommand("net", netDisp, "Manage network namespaces.")
	disp.AddCommand("container", containerDisp, "Manage lightweight containers (systemd-nspawn).")

	// Run the root dispatcher
	vclip.Main(context.Background(), disp, os.Args[1:])
}
