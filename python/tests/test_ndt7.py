# SPDX-License-Identifier: GPL-3.0-or-later

import pytest

from npte import NpteNdt7Client, NpteNdt7Server


# --- NpteNdt7Server ---


def test_server_is_frozen():
    s = NpteNdt7Server(
        binary="/bin/ndt-server", datadir="/data", cert="/c.pem", key="/k.pem"
    )
    with pytest.raises(AttributeError):
        s.binary = "other"  # type: ignore[misc]


def test_server_defaults():
    s = NpteNdt7Server(
        binary="/bin/ndt-server", datadir="/data", cert="/c.pem", key="/k.pem"
    )
    assert s.cleartext_port == 8081
    assert s.tls_port == 4444


def test_server_argv():
    s = NpteNdt7Server(
        binary="/bin/ndt-server", datadir="/data", cert="/c.pem", key="/k.pem"
    )
    assert s.server_argv() == [
        "/bin/ndt-server",
        "-ndt7_addr_cleartext",
        ":8081",
        "-ndt7_addr",
        ":4444",
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
        "/c.pem",
        "-key",
        "/k.pem",
        "-datadir",
        "/data",
        "-compress-results=false",
    ]


# --- NpteNdt7Client ---


def test_client_is_frozen():
    c = NpteNdt7Client(binary="/bin/ndt7-client")
    with pytest.raises(AttributeError):
        c.scheme = "wss"  # type: ignore[misc]


def test_client_defaults():
    c = NpteNdt7Client(binary="/bin/ndt7-client")
    assert c.scheme == "ws"
    assert c.port == 8081
    assert c.timeout == 30


def test_client_argv():
    c = NpteNdt7Client(binary="/bin/ndt7-client")
    assert c.client_argv("172.16.2.2") == [
        "/usr/bin/time",
        "-p",
        "/bin/ndt7-client",
        "-server",
        "172.16.2.2:8081",
        "-scheme",
        "ws",
        "-no-verify",
        "-download",
        "-upload=false",
        "-format=json",
    ]


def test_client_argv_tls():
    c = NpteNdt7Client(binary="/bin/ndt7-client", scheme="wss", port=4444)
    assert c.client_argv("172.16.2.2") == [
        "/usr/bin/time",
        "-p",
        "/bin/ndt7-client",
        "-server",
        "172.16.2.2:4444",
        "-scheme",
        "wss",
        "-no-verify",
        "-download",
        "-upload=false",
        "-format=json",
    ]
