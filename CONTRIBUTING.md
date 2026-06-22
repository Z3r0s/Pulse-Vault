# Contributing

Thanks for helping improve Pulse-Vault.

Official site: [dnspulse.org](https://dnspulse.org). Packaged downloads will be hosted there later.
For now, install from source in this repository.

## Development Setup

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
PULSEVAULT_TEST_FAST_KDF=1 python -m unittest discover -s tests -v
```

Tests use a fast Scrypt setting through `PULSEVAULT_TEST_FAST_KDF=1`. GitHub
Actions sets the same variable automatically. You do not need your own server for
that; GitHub provides the runner machine.

## Guidelines

- Keep the app local-first and offline.
- Avoid adding network services or telemetry.
- Add tests for vault format, password rotation, and migration changes.
- Keep security claims precise and verifiable.
- Do not commit real vault files or recovery keys.
