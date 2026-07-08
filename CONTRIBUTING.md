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
  3. Bump `__version__` in `src/pulsevault/__init__.py`.
  4. Update the latest `<release>` entry (and add historical if new) in `packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml`.
  5. Commit, `git tag vX.Y.Z`, push tag.

## Version Sync Notes (for packaging / App Store readiness)

- **Single source of truth:** `__version__` lives only in `src/pulsevault/__init__.py`.
- **Always sync before release/tag:**
  - `src/pulsevault/__init__.py` (the `__version__`)
  - `CHANGELOG.md` (finalize Unreleased section)
  - `packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml` (update/add `<release version="X.Y.Z" date="...">`)
- pyproject.toml pulls version dynamically: `version = {attr = "pulsevault.__version__"}`
- Desktop metadata, CLI `--version`, GUI About, release workflow, and AppStream all derive from these.
- After version bump: locally validate with `desktop-file-validate packaging/linux/pulse-vault.desktop` and `appstream-util validate-relax packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml` (or use CI).
- For Snap/Flatpak: ensure the metainfo version matches the snap version string.
- Test post-bump: `pip install -e .` then `pulse-vault --version` (and `pulse-vault-cli --help` if available via entry points).
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