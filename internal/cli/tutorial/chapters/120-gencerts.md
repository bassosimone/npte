# Generating test certificates

When testing HTTPS clients or servers inside an npte lab, you need a
TLS certificate that the server can present and the client can verify.
`npte gencerts` generates a self-signed certificate and private key
for this purpose.

## Quick usage

Generate a certificate covering `127.0.0.1` (the default):

    npte gencerts

This writes `cert.pem` and `key.pem` to the current directory. The
certificate is valid for one year.

## Custom SANs

To issue a certificate for specific IP addresses or DNS names, use
the `--ip-addr` and `--dns-name` flags. Both are repeatable:

    npte gencerts --ip-addr 10.0.0.1 --ip-addr 10.0.0.2 --dns-name server.lab

When `--ip-addr` is omitted, it defaults to `127.0.0.1`. When
`--dns-name` is omitted, it defaults to the first IP address.

## Output directory

Use `-C` to write the files somewhere other than the current
directory:

    npte gencerts -C /tmp/certs

The directory is created if it does not exist.

## Notes

The generated certificates are self-signed and meant exclusively for
local testing. The client under test must be configured to trust
`cert.pem` directly (e.g. via a custom CA pool or by disabling
certificate verification) — it will not chain to any system root.

This command does not require root and does not touch the network.
