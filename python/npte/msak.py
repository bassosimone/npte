"""Wrappers for starting ``msak-server`` and running ``msak-client``."""

# SPDX-License-Identifier: GPL-3.0-or-later

import uuid

from .executor import RunningProcess, TerminatedProcess
from .lab import NpteLab
from .ports import MSAK_CLEARTEXT_PORT, MSAK_TLS_PORT


def start_msak_server(
    lab: NpteLab,
    *,
    binary: str,
    datadir: str,
    cert: str,
    key: str,
    cleartext_port: int = MSAK_CLEARTEXT_PORT,
    tls_port: int = MSAK_TLS_PORT,
) -> RunningProcess:
    """Start an msak-server in the lab's server namespace (cleartext + TLS)."""
    return lab.server.start(
        [
            binary,
            "-ws_addr",
            f"{lab.server.addr}:{cleartext_port}",
            "-wss_addr",
            f"{lab.server.addr}:{tls_port}",
            "-prometheusx.listen-address",
            "127.0.0.1:9991",
            "-cert",
            cert,
            "-key",
            key,
            "-datadir",
            datadir,
        ]
    )


def run_msak_client(
    lab: NpteLab,
    *,
    binary: str,
    cc: str,
    scheme: str = "ws",
    duration: str = "10s",
    port: int = MSAK_CLEARTEXT_PORT,
    timeout: float = 30,
) -> TerminatedProcess:
    """Run an msak-client download test in the lab's client namespace."""
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
            "-streams",
            "1",
            "-cc",
            cc,
            "-duration",
            duration,
            "-download",
            "-upload=false",
            "-mid",
            str(uuid.uuid7()),
        ],
        timeout=timeout,
    )
