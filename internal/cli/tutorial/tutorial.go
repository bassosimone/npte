// SPDX-License-Identifier: GPL-3.0-or-later

// Package tutorial implements the tutorial subcommand.
package tutorial

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/textwrap"
	"github.com/bassosimone/vflag"
	"github.com/charmbracelet/glamour"
)

//go:embed chapters/*.md
var chaptersFS embed.FS

// prefixRe strips a leading "NNN-" ordering prefix from chapter filenames.
var prefixRe = regexp.MustCompile(`^\d+-`)

// Main is the main of the tutorial subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte tutorial", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Display npte tutorial chapters, rendered as styled markdown. Without " +
			"arguments, lists the available chapters with their titles. With a " +
			"chapter slug, renders that single chapter. With the special slug " +
			"`all`, renders every chapter in order.",
	)
	usage.PositionalArgumentsUsage = "[chapter-slug | all]"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	chapters := loadChapters()

	if len(fset.Args()) <= 0 {
		render(env.Stdout, buildTOC(chapters))
		return nil
	}

	slug := fset.Args()[0]
	if slug == "all" {
		render(env.Stdout, concatChapters(chapters))
		return nil
	}

	for _, ch := range chapters {
		if ch.slug == slug {
			render(env.Stdout, ch.body)
			return nil
		}
	}

	logx.Error("npte tutorial: unknown chapter %q", slug)
	logx.Error("npte tutorial: run `npte tutorial` to list chapters")
	env.Exit(1)
	return nil
}

// chapter is one embedded tutorial chapter.
type chapter struct {
	slug  string
	title string
	body  string
}

// loadChapters reads every embedded chapter in alphabetical (i.e. numeric-prefix) order.
func loadChapters() []chapter {
	paths := runtimex.LogFatalOnError1(fs.Glob(chaptersFS, "chapters/*.md"))
	chapters := make([]chapter, 0, len(paths))
	for _, path := range paths {
		body := string(runtimex.LogFatalOnError1(chaptersFS.ReadFile(path)))
		name := strings.TrimSuffix(strings.TrimPrefix(path, "chapters/"), ".md")
		slug := prefixRe.ReplaceAllString(name, "")
		chapters = append(chapters, chapter{
			slug:  slug,
			title: extractTitle(body, slug),
			body:  body,
		})
	}
	return chapters
}

// extractTitle returns the first `# ` heading in body, or fallback if there is none.
func extractTitle(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}

// tocWrapWidth is the source-line width used for prose paragraphs in the TOC.
// Matches the chapters under chapters/, which wrap at ~68 characters.
const tocWrapWidth = 68

// buildTOC returns an auto-generated table-of-contents markdown for the given chapters.
func buildTOC(chapters []chapter) string {
	var sb strings.Builder
	sb.WriteString("# npte tutorial\n\n")
	sb.WriteString(textwrap.Do(
		"npte (Network Performance Testing Environment) creates "+
			"isolated Linux network namespaces, wires them together "+
			"with veth pairs, and shapes the links with `tc`/`netem` — "+
			"so you can test how a client behaves on a realistic "+
			"access link without touching real hardware or a second "+
			"machine.",
		tocWrapWidth, "",
	))
	sb.WriteString("\n\n")
	sb.WriteString(textwrap.Do(
		"The central abstraction is a **network namespace**: a "+
			"private, kernel-level network stack with its own "+
			"interfaces, addresses, routes, and iptables state. npte's "+
			"commands are small primitives for creating, connecting, "+
			"addressing, routing, and shaping these. The chapters "+
			"below build up from a single empty namespace to a "+
			"`client — router — server` star with realistic traffic "+
			"shaping on the access link, and end with the queueing "+
			"dynamics that make a shaped link feel either tolerable or "+
			"broken under load.",
		tocWrapWidth, "",
	))
	sb.WriteString("\n\n")
	if len(chapters) <= 0 {
		sb.WriteString("No chapters are currently embedded.\n")
		return sb.String()
	}
	sb.WriteString("## Chapters\n\n")
	for _, ch := range chapters {
		fmt.Fprintf(&sb, "- `%s` — %s\n", ch.slug, ch.title)
	}
	sb.WriteString("\n")
	sb.WriteString(textwrap.Do(
		"Read a chapter with `npte tutorial <slug>`, or read every "+
			"chapter in order with `npte tutorial all`. The chapters "+
			"build on each other; read in order if you are starting "+
			"fresh.",
		tocWrapWidth, "",
	))
	sb.WriteString("\n")
	return sb.String()
}

// concatChapters joins every chapter body with a horizontal rule separator.
func concatChapters(chapters []chapter) string {
	var sb strings.Builder
	for i, ch := range chapters {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(ch.body)
	}
	return sb.String()
}

// render pipes markdown through glamour and writes the result to w.
func render(w io.Writer, body string) {
	r := runtimex.LogFatalOnError1(glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	))
	rendered := runtimex.LogFatalOnError1(r.Render(body))
	fmt.Fprint(w, rendered)
}
