# Contributing

Thanks for helping improve Pulse-Vault.

Official site: [dnspulse.org](https://dnspulse.org). Packaged downloads will be hosted there later.

For full installation instructions (including Linux `tkinter` requirements), see [INSTALL.md](INSTALL.md).

## Development Setup

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
PULSEVAULT_TEST_FAST_KDF=1 python -m unittest discover -s tests -v
```

Tests use the `fast` Scrypt profile through `PULSEVAULT_TEST_FAST_KDF=1` so the suite
finishes quickly on developer machines. CI uses the same variable.

To regenerate golden crypto vectors after format or KDF changes:

```bash
PULSEVAULT_SCRYPT_PROFILE=fast python tests/generate_vectors.py
python tests/generate_vectors.py --profile standard
```

Optional fuzz dependencies:

```bash
pip install hypothesis
```

## Changelog & Release Process

- Always update the `## [Unreleased]` section in [CHANGELOG.md](CHANGELOG.md) before submitting a PR (use standard Keep a Changelog headings: Added, Changed, Fixed, Security, etc.).
- The PR template will remind you.
- To cut a release:
  1. Finalize `[Unreleased]` → rename it to `## [X.Y.Z] - YYYY-MM-DD`.
  2. Add a fresh `## [Unreleased]` at the top.
  3. Bump `__version__` in `src/pulsevault/__init__.py` (and metainfo if needed).
  4. Commit, `git tag vX.Y.Z`, push tag.
- The `.github/workflows/release.yml` workflow will automatically:
  - Run the full test suite
  - Build sdist + wheel
  - Extract the matching CHANGELOG section as release notes
  - Attach the advanced security property fuzz report (from Hypothesis tests)
  - Create the GitHub Release with assets + checksums
- See also the [GitHub Releases page](https://github.com/Z3r0s/Pulse-Vault/releases) and `docs/DOWNLOADS.md`.

This keeps changelogs as the single source of truth surfaced directly on GitHub.

## Guidelines

- Keep the app local-first and offline.
- Avoid adding network services or telemetry.
- Add tests for vault format, password rotation, and migration changes.
- Keep security claims precise and verifiable.
- Do not commit real vault files or recovery keys.