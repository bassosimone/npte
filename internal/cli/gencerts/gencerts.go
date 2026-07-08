// SPDX-License-Identifier: GPL-3.0-or-later

// Package gencerts implements the gencerts subcommand.
package gencerts

import (
	"context"
	"net"
	"path/filepath"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/pkitest"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// Main is the main of the gencerts subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	var (
		dnsNames  []string
		ipAddrs   []string
		outputDir = "."
	)

	fset := vflag.NewFlagSet("npte gencerts", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Generates a self-signed TLS certificate and private key for "+
			"testing. The output files are cert.pem and key.pem, written "+
			"to the directory specified by -C (default: current directory).",
		"By default the certificate covers 127.0.0.1 as both an IP SAN "+
			"and a DNS SAN. Use --ip-addr and --dns-name to override.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.StringSliceVar(&dnsNames, 0, "dns-name", "Add a DNS SAN to the certificate (repeatable).")
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringSliceVar(&ipAddrs, 0, "ip-addr", "Add an IP SAN to the certificate (repeatable).")
	fset.StringVar(&outputDir, 'C', "directory", "Generate files in `DIR` (default: @DEFAULT_VALUE@).")
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	// Fallback to sensible defaults when no SANs are specified
	if len(ipAddrs) <= 0 {
		ipAddrs = []string{"127.0.0.1"}
	}
	if len(dnsNames) <= 0 {
		dnsNames = []string{ipAddrs[0]}
	}

	// Parse IP addresses
	ips := make([]net.IP, 0, len(ipAddrs))
	for _, candidate := range ipAddrs {
		ip := net.ParseIP(candidate)
		if ip == nil {
			logx.Error("npte gencerts: invalid IP address: %s", candidate)
			env.Exit(2)
			return nil
		}
		ips = append(ips, ip)
	}

	config := &pkitest.SelfSignedCertConfig{
		CommonName:   dnsNames[0],
		DNSNames:     dnsNames,
		IPAddrs:      ips,
		Organization: []string{"npte"},
	}

	logx.Details("generating cert.pem and key.pem inside the `%s` directory", outputDir)
	env.LogFatalOnError0(env.MkdirAll(outputDir, 0700))
	logx.Command("mkdir -p %s", outputDir)

	// [pkitest.MustNewSelfSignedCert] panics on error but this is fine: once
	// the inputs above are validated, cert generation cannot fail
	ssc := pkitest.MustNewSelfSignedCert(config)
	mustWriteFiles(env, ssc, outputDir)
	return nil
}

// TODO(bassosimone): the 0600 mode only applies when the file is created;
// overwriting an existing key.pem keeps its current permission bits, so a
// pre-existing 0644 file would hold the fresh private key world-readable.
// Consider removing the target before writing (or chmod after), and extend
// the test's WriteFile mock to record and pin the requested mode.
func mustWriteFiles(env *testable.Environ, ssc *pkitest.SelfSignedCert, outputDir string) {
	env.LogFatalOnError0(env.WriteFile(filepath.Join(outputDir, "cert.pem"), ssc.CertPEM, 0600))
	logx.Details("certificate file: %s", filepath.Join(outputDir, "cert.pem"))

	env.LogFatalOnError0(env.WriteFile(filepath.Join(outputDir, "key.pem"), ssc.KeyPEM, 0600))
	logx.Details("private key file: %s", filepath.Join(outputDir, "key.pem"))
}
