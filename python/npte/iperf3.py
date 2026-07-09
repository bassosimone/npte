"""Command-line builders for ``iperf3`` server and client."""

# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses

from .ports import IPERF3_PORT


@dataclasses.dataclass(frozen=True)
class NpteIperf3Server:
    """Configuration for an iperf3 server."""

    binary: str = "iperf3"
    port: int = IPERF3_PORT

    def server_argv(self) -> list[str]:
        return [self.binary, "-s", "-J", "-p", str(self.port)]


@dataclasses.dataclass(frozen=True)
class NpteIperf3Client:
    """Configuration for an iperf3 reverse-mode download client."""

    binary: str = "iperf3"
    cc: str = "cubic"
    duration: str = "10"
    port: int = IPERF3_PORT
    timeout: float = 30

    def client_argv(self, server_addr: str) -> list[str]:
        return [
            "/usr/bin/time",
            "-p",
            self.binary,
            "-c",
            server_addr,
            "-p",
            str(self.port),
            "-R",
            "-J",
            "--get-server-output",
            "-t",
            self.duration,
            "-C",
            self.cc,
        ]
