# SPDX-License-Identifier: GPL-3.0-or-later

import pytest

from npte import NpteMsakClient, NpteMsakServer


# --- NpteMsakServer ---


def test_server_is_frozen():
    s = NpteMsakServer(
        binary="/bin/msak-server", datadir="/data", cert="/c.pem", key="/k.pem"
    )
    with pytest.raises(AttributeError):
        s.binary = "other"  # type: ignore[misc]


def test_server_defaults():
    s = NpteMsakServer(
        binary="/bin/msak-server", datadir="/data", cert="/c.pem", key="/k.pem"
    )
    assert s.cleartext_port == 8080
    assert s.tls_port == 4443


def test_server_argv():
    s = NpteMsakServer(
        binary="/bin/msak-server", datadir="/data", cert="/c.pem", key="/k.pem"
    )
    assert s.server_argv() == [
        "/bin/msak-server",
        "-ws_addr",
        ":8080",
        "-wss_addr",
        ":4443",
        "-prometheusx.listen-address",
        "127.0.0.1:9991",
        "-cert",
        "/c.pem",
        "-key",
        "/k.pem",
        "-datadir",
        "/data",
    ]


# --- NpteMsakClient ---


def test_client_is_frozen():
    c = NpteMsakClient(binary="/bin/msak-client")
    with pytest.raises(AttributeError):
        c.cc = "bbr"  # type: ignore[misc]


def test_client_defaults():
    c = NpteMsakClient(binary="/bin/msak-client")
    assert c.cc == "cubic"
    assert c.scheme == "ws"
    assert c.duration == "10s"
    assert c.port == 8080
    assert c.timeout == 30


def test_client_argv(monkeypatch):
    monkeypatch.setattr("npte.msak.uuid.uuid7", lambda: "fake-mid-uuid")
    c = NpteMsakClient(binary="/bin/msak-client", cc="bbr")
    assert c.client_argv("172.16.2.2") == [
        "/usr/bin/time",
        "-p",
        "/bin/msak-client",
        "-server",
        "172.16.2.2:8080",
        "-scheme",
        "ws",
        "-no-verify",
        "-streams",
        "1",
        "-cc",
        "bbr",
        "-duration",
        "10s",
        "-download",
        "-upload=false",
        "-mid",
        "fake-mid-uuid",
    ]


def test_client_argv_tls(monkeypatch):
    monkeypatch.setattr("npte.msak.uuid.uuid7", lambda: "fake-mid-uuid")
    c = NpteMsakClient(binary="/bin/msak-client", scheme="wss", port=4443)
    assert c.client_argv("172.16.2.2") == [
        "/usr/bin/time",
        "-p",
        "/bin/msak-client",
        "-server",
        "172.16.2.2:4443",
        "-scheme",
        "wss",
        "-no-verify",
        "-streams",
        "1",
        "-cc",
        "cubic",
        "-duration",
        "10s",
        "-download",
        "-upload=false",
        "-mid",
        "fake-mid-uuid",
    ]
