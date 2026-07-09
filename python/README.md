# npte Python Package

Python bindings for driving [npte](../README.md) labs from scripts.
The package shells out to `sudo npte ...` under the hood, so all the
same Linux/root constraints apply.

## Requirements

- Python >= 3.14
- [uv](https://docs.astral.sh/uv/) (package manager)
- `npte` installed and on `PATH` (see root README)
- Passwordless sudoers rules installed via `npte sudoers`

## Install

The Debian package produced by `makedeb.bash` ships the Python package
under `/usr/lib/python3/dist-packages`, so it is importable system-wide
with no extra step.

For development, install as an editable package:

```bash
cd python
uv sync --dev
```

## Usage

### Grid sweep

Build the measurement tools, then sweep rates, RTTs, and
congestion-control algorithms across iperf3, MSAK, and ndt7:

```python
from npte import (
    NpteCell,
    NpteGrid,
    NpteLab,
    NpteTools,
    npte_symmetric_shaping_matrix,
)

tools = NpteTools()
tools.generate_certs()
tools.build_msak()
tools.build_ndt7()

with NpteLab() as lab:
    grid = NpteGrid(lab)
    grid.add_server(tools.iperf3_server())
    grid.add_server(tools.msak_server())
    grid.add_server(tools.ndt7_server())

    for shaping in npte_symmetric_shaping_matrix(
        rates=["100mbit"],
        rtts_ms=[5, 25],
    ):
        for cc in ["bbr", "cubic"]:
            cell = NpteCell()
            cell.set_download(shaping)
            cell.set_upload(shaping)
            cell.add_client(tools.iperf3_client(cc=cc))
            cell.add_client(tools.msak_client(cc=cc))
            cell.add_client(tools.ndt7_client())
            grid.add_cell(cell)

    for result in grid.run():
        print(f"exit={result.exitcode} dir={result.proc_dir}")
```

`NpteLab` is a context manager that creates the three-namespace
topology (`client`, `router`, `server`) on entry and destroys it
on exit.

`NpteTools` builds Go measurement binaries (`ndt-server`, `msak-server`,
`ndt7-client`, etc.) into `~/.npte/bin/` and generates TLS certificates
into `~/.npte/certs/`. The factory methods return frozen dataclasses that
know how to produce the right `argv` for each tool.

`npte_symmetric_shaping_matrix` generates an `NpteShaping` for each
`(rate, rtt)` combination, splitting the RTT equally between the two
directions.

`NpteGrid` automates the loop: start each server once, then for every
cell apply the shaping, run each client, and collect results. Each
client run is recorded in a per-process directory under `~/.npte/data/`.

### Power-saving check

```python
from npte import npte_energy_performance_preferences

prefs = npte_energy_performance_preferences()
if prefs != {"performance"}:
    print(f"Warning: CPU governor(s) set to {prefs}")
```

## Public API

Import from the package, not from internal modules:

```python
# correct
from npte import NpteLab, NpteGrid

# wrong -- module names may change
from npte.lab import NpteLab
```

All public symbols use the `Npte` prefix (classes) or `npte_` prefix
(functions). See `npte/__init__.py` for the full list.

## Development

```bash
uv run pytest                   # run tests
uv run ruff check               # lint
uv run ruff format --check      # format check
uv run pyright                  # type check
```

## License

```
SPDX-License-Identifier: GPL-3.0-or-later
```
