"""Wrappers for starting ``ndt-server`` and running ``ndt7-client``."""

# SPDX-License-Identifier: GPL-3.0-or-later

from .executor import RunningProcess, TerminatedProcess
from .lab import NpteLab
from .ports import NDT7_CLEARTEXT_PORT, NDT7_TLS_PORT


def start_ndt_server(
    lab: NpteLab,
    *,
    binary: str,
    datadir: str,
    cert: str,
    key: str,
    cleartext_port: int = NDT7_CLEARTEXT_PORT,
    tls_port: int = NDT7_TLS_PORT,
) -> RunningProcess:
    """Start an ndt-server in the lab's server namespace (cleartext + TLS)."""
    return lab.server.start(
        [
            binary,
            "-ndt7_addr_cleartext",
            f":{cleartext_port}",
            "-ndt7_addr",
            f":{tls_port}",
            "-ndt5_addr",
            "127.0.0.1:9301",
            "-ndt5_ws_addr",
            "127.0.0.1:9302",
            "-ndt5_wss_addr",
            "127.0.0.1:9310",
            "-health_addr",
            "127.0.0.1:9800",
            "-prometheusx.listen-address",
            "127.0.0.1:9990",
            "-cert",
            cert,
            "-key",
            key,
            "-datadir",
            datadir,
            "-compress-results=false",
        ]
    )


def run_ndt7_client(
    lab: NpteLab,
    *,
    binary: str,
    scheme: str = "ws",
    port: int = NDT7_CLEARTEXT_PORT,
    timeout: float = 30,
) -> TerminatedProcess:
    """Run an ndt7-client download test in the lab's client namespace."""
    return lab.client.run(
        [
            "/usr/bin/time",
            "-p",
            binary,
            "-server",
            f"{lab.server.addr}:{port}",
            "-scheme",
            scheme,
            "-no-verify",
            "-download",
            "-upload=false",
            "-format=json",
        ],
        timeout=timeout,
    )
