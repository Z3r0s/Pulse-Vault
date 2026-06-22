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

## Guidelines

- Keep the app local-first and offline.
- Avoid adding network services or telemetry.
- Add tests for vault format, password rotation, and migration changes.
- Keep security claims precise and verifiable.
- Do not commit real vault files or recovery keys.