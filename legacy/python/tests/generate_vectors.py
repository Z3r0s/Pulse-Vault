"""One-shot maintainer script to regenerate tests/vectors golden files.

Does not persist the test password on disk — consumers import VECTOR_TEST_PASSWORD
from vector_constants and re-derive keys the same way this script does.
"""

import argparse
import io
import json
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

import pulsevault.core.crypto as crypto  # noqa: E402
from vector_constants import VECTOR_TEST_PASSWORD  # noqa: E402

VECTOR_DIR = Path(__file__).resolve().parent / "vectors"
VECTOR_DIR.mkdir(exist_ok=True)

SALT = bytes(range(16))
PLAINTEXT = b"Pulse-Vault vector payload\n"
METADATA_JSON = (
    b'{"version":5,"created_at":1,"updated_at":1,'
    b'"files":{"sample.txt":{"name":"sample.txt","size":29}}}'
)

_random_counter = 0


def fake_urandom(size: int) -> bytes:
    global _random_counter
    out = bytearray()
    while len(out) < size:
        out.append((_random_counter + len(out)) % 256)
    _random_counter += 1
    return bytes(out[:size])


def _write_json(path: Path, payload: dict) -> None:
    """Write vector metadata. Callers must never put secrets in payload."""
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="Generate Pulse-Vault crypto test vectors.")
    parser.add_argument(
        "--profile",
        default="fast",
        choices=sorted(crypto.SCRYPT_PROFILES),
        help="Scrypt profile used for the generated vectors.",
    )
    args = parser.parse_args()

    os.environ["PULSEVAULT_SCRYPT_PROFILE"] = args.profile
    import importlib

    importlib.reload(crypto)

    suffix = args.profile
    crypto.os.urandom = fake_urandom

    # Derive in memory only — do not serialize the passphrase to vector files.
    key64 = crypto.derive_key_v3(VECTOR_TEST_PASSWORD, SALT)
    chacha_key, aes_key = crypto.split_v3_key(key64)

    scrypt_vector = {
        "profile": suffix,
        "salt_hex": SALT.hex(),
        "scrypt_n": crypto.SCRYPT_N,
        "scrypt_r": crypto.SCRYPT_R,
        "scrypt_p": crypto.SCRYPT_P,
        "key64_hex": key64.hex(),
        "chacha_key_hex": chacha_key.hex(),
        "aes_key_hex": aes_key.hex(),
    }
    _write_json(VECTOR_DIR / f"scrypt_{suffix}.json", scrypt_vector)

    c_nonce, a_nonce, meta_ct = crypto.encrypt_data_v3(key64, METADATA_JSON)
    metadata_vector = {
        "profile": suffix,
        "salt_hex": SALT.hex(),
        "plaintext_hex": METADATA_JSON.hex(),
        "chacha_nonce_hex": c_nonce.hex(),
        "aes_nonce_hex": a_nonce.hex(),
        "ciphertext_hex": meta_ct.hex(),
    }
    _write_json(VECTOR_DIR / f"metadata_v3_{suffix}.json", metadata_vector)

    global _random_counter
    _random_counter = 0
    stream_out = io.BytesIO()
    crypto.encrypt_stream_v5(key64, io.BytesIO(PLAINTEXT), stream_out, compress=True)
    stream_bytes = stream_out.getvalue()
    (VECTOR_DIR / f"stream_v5_{suffix}.bin").write_bytes(stream_bytes)

    stream_vector = {
        "profile": suffix,
        "salt_hex": SALT.hex(),
        "plaintext_hex": PLAINTEXT.hex(),
        "stream_hex": stream_bytes.hex(),
        "stream_size": len(stream_bytes),
    }
    _write_json(VECTOR_DIR / f"stream_v5_{suffix}.json", stream_vector)

    print(f"Wrote vectors to {VECTOR_DIR}")


if __name__ == "__main__":
    main()
