# Python Package

House rules for the `python/` subtree.

## Tooling

- Use `uv run` for everything — never `pip`, `python -m`, or bare `pytest`.
- Quality gates: `uv run ruff check`, `uv run ruff format --check`, `uv run pyright`, `uv run pytest`.
- Tools configuration lives in the workspace root so that gates behave identically for every tool
- We define dependencies in this directory for the same reason

## Style

- `len(x) <= 0` not `== 0` for emptiness checks.
- Prefer keyword-only args (`*, ...`) for clarity at call sites.
- Public API: `Npte`-prefixed classes, `npte_`-prefixed functions, all exported from `__init__.py`.
- Import from the package (`from npte import NpteLab`), not from internal modules (`from npte.lab import ...`).

## Testing

- One test file per module.
- Prefer protocol doubles (e.g., `FakeExecutor`) over monkey patching for in-house code.
- Use monkey patching only for foreign dependencies (`time.sleep`, `uuid.uuid7`, `glob.glob`, etc.).
- Assert exact equality for deterministic outputs.

## Documentation

- When changing the public API, cross-check: `__init__.py` (exports + docstring examples), `python/README.md` (usage examples), and `__all__`.
