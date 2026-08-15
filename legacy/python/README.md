# Old Python code

This is leftover from before the Go rewrite. Don't `pip install` it. Don't add features. CI doesn't run it.

The real app is [`gui-go/`](../../gui-go/).

## Vectors

Go still reads [`tests/vectors/`](../../tests/vectors/) from the repo root (not this folder).

`bench_lifecycle.py` is only for the Go vs Python speed test. Not an install.

## Layout

| Path | Contents |
| --- | --- |
| `src/pulsevault/` | Retired Python package |
| `main.py` | Old entry point |
| `tests/` | Retired Python unit tests |
| `packaging/` | Old `verify-build.py` and PyInstaller `build-binaries.ps1` |
| `pyproject.toml` / `requirements.txt` | Archive-only metadata (not published) |

## Product entry points (Go)

| Binary | Path |
| --- | --- |
| Desktop GUI | `gui-go/` |
| CLI | `gui-go/cmd/pulse-vault` |

See the root [README.md](../../README.md) and [INSTALL.md](../../INSTALL.md).
