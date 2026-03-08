// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/charmbracelet/glamour"
)

//go:embed docs/tutorial/*.md
var tutorialFS embed.FS

func tutorialMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte tutorial", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Display the npte tutorial. Without arguments, shows the tutorial overview "+
			"and lists all the available tutorial chapters.",
		"Available chapters:",
		"- quickstart",
		"- namespaces",
		"- containers",
		"- netem",
		"- browser",
	)
	usage.PositionalArgumentsUsage = "[chapter]"
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	filename := "docs/tutorial/README.md"
	if len(fset.Args()) > 0 {
		filename = fmt.Sprintf("docs/tutorial/%s.md", fset.Args()[0])
	}

	content, err := tutorialFS.ReadFile(filename)
	if err != nil {
		logError("npte tutorial: unknown chapter %q", fset.Args()[0])
		logError("npte tutorial: available chapters: quickstart, namespaces, containers, netem, browser")
		env.Exit(1)
	}

	r := runtimex.LogFatalOnError1(glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	))
	rendered := runtimex.LogFatalOnError1(r.Render(string(content)))

	fmt.Fprint(env.Stdout, rendered)
	return nil
}
