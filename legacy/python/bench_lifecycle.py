#!/usr/bin/env python3
"""Time the archived Python vault/crypto on the same work as the Go compare test.

Prints one JSON object to stdout. Not a product path — used only so the Go
suite can measure a real head-to-head. Requires the `cryptography` package.
"""
from __future__ import annotations

import json
import os
import sys
import tempfile
import time
from io import BytesIO
from pathlib import Path

# Allow `python bench_lifecycle.py` from this folder or the repo root.
HERE = Path(__file__).resolve().parent
SRC = HERE / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))

os.environ.setdefault("PULSEVAULT_SCRYPT_PROFILE", "fast")
os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

from pulsevault.core.crypto import (  # noqa: E402
    SALT_SIZE,
    derive_key_scrypt,
    decrypt_stream_v5,
    encrypt_stream_v5,
    scrypt_params_for_profile,
)
from pulsevault.core.vault import EncryptedVault  # noqa: E402


def _ms(fn, loops: int) -> float:
    best = None
    for _ in range(loops):
        t0 = time.perf_counter()
        fn()
        dt = (time.perf_counter() - t0) * 1000.0
        best = dt if best is None or dt < best else best
    return best


def _payload(n: int) -> bytes:
    # Same repeating pattern the Go compare test uses so compression work matches.
    chunk = b"pulse-vault-benchmark-data-"
    return (chunk * ((n // len(chunk)) + 1))[:n]


def _incompressible(n: int) -> bytes:
    # Deterministic high-entropy bytes (SHA-256 expand). Same seed as the Go test.
    import hashlib

    block = b"seed-pulse-vault-compare-v1"
    out = bytearray()
    while len(out) < n:
        block = hashlib.sha256(block).digest()
        out.extend(block)
    return bytes(out[:n])


def main() -> int:
    payload_4 = _payload(4 * 1024 * 1024)
    payload_2 = payload_4[: 2 * 1024 * 1024]
    payload_1 = payload_4[: 1 * 1024 * 1024]
    incomp_4 = _incompressible(4 * 1024 * 1024)
    n, r, p = scrypt_params_for_profile("fast")
    salt = os.urandom(SALT_SIZE)
    key = derive_key_scrypt("GoVsPythonCompare!!", salt, n, r, p)

    def stream_enc():
        out = BytesIO()
        encrypt_stream_v5(key, BytesIO(payload_4), out, compress=True)

    def stream_enc_incomp():
        out = BytesIO()
        encrypt_stream_v5(key, BytesIO(incomp_4), out, compress=True)

    enc_buf = BytesIO()
    encrypt_stream_v5(key, BytesIO(payload_4), enc_buf, compress=True)
    enc_bytes = enc_buf.getvalue()

    def stream_dec():
        decrypt_stream_v5(key, BytesIO(enc_bytes), BytesIO())

    def four_seq():
        for _ in range(4):
            encrypt_stream_v5(key, BytesIO(payload_1), BytesIO(), compress=True)

    loops = 3
    ops = {
        "stream_encrypt_4mib_ms": _ms(stream_enc, loops),
        "stream_encrypt_4mib_incompressible_ms": _ms(stream_enc_incomp, loops),
        "stream_decrypt_4mib_ms": _ms(stream_dec, loops),
        "parallel_4x_1mib_encrypt_ms": _ms(four_seq, loops),
    }

    with tempfile.TemporaryDirectory(prefix="pv_py_bench_") as td:
        td_path = Path(td)
        vault_path = td_path / "bench.pulsevault"
        src = td_path / "blob.bin"
        src.write_bytes(payload_2)
        v = EncryptedVault(vault_path)
        v.create("GoVsPythonCompare!!", scrypt_profile="fast")

        def add():
            v.add_file(src, overwrite=True)

        ops["vault_add_2mib_ms"] = _ms(add, loops)

        def extract():
            v.extract_file("blob.bin", td_path / "out", overwrite=True)

        ops["vault_extract_2mib_ms"] = _ms(extract, loops)

    print(json.dumps({"impl": "python", "ops": ops}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
