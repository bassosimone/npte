"""Wrappers for starting ``iperf3`` server and running ``iperf3`` client."""

# SPDX-License-Identifier: GPL-3.0-or-later

from .executor import RunningProcess, TerminatedProcess
from .lab import NpteLab
from .ports import IPERF3_PORT


def start_iperf3_server(
    lab: NpteLab,
    *,
    binary: str = "iperf3",
    port: int = IPERF3_PORT,
) -> RunningProcess:
    """Start an iperf3 server in the lab's server namespace."""
    return lab.server.start(
        [
            binary,
            "-s",
            "-J",
            "-p",
            str(port),
        ]
    )


def run_iperf3_client(
    lab: NpteLab,
    *,
    binary: str = "iperf3",
    cc: str,
    duration: str = "10",
    port: int = IPERF3_PORT,
    timeout: float = 30,
) -> TerminatedProcess:
    """Run an iperf3 reverse-mode download test in the lab's client namespace."""
    return lab.client.run(
        [
            "/usr/bin/time",
            "-p",
            binary,
            "-c",
            lab.server.addr,
            "-p",
            str(port),
            "-R",
            "-J",
            "--get-server-output",
            "-t",
            duration,
            "-C",
            cc,
        ],
        timeout=timeout,
    )
