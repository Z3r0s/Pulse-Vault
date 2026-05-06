# Contributing

Thanks for helping improve Pulse-Vault.

## Development Setup

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e .
python -m unittest
```

## Guidelines

- Keep the app local-first and offline.
- Avoid adding network services or telemetry.
- Add tests for vault format, password rotation, and migration changes.
- Keep security claims precise and verifiable.
- Do not commit real vault files or recovery keys.
