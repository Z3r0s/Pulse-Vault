"""One-shot maintainer script to regenerate tests/vectors golden files."""

import io
import json
import os
import sys
from pathlib import Path

os.environ["PULSEVAULT_SCRYPT_PROFILE"] = "fast"

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

import pulsevault.core.crypto as crypto  # noqa: E402

VECTOR_DIR = Path(__file__).resolve().parent / "vectors"
VECTOR_DIR.mkdir(exist_ok=True)

PASSWORD = "vector-test-password!"
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


def main():
    crypto.os.urandom = fake_urandom

    key64 = crypto.derive_key_v3(PASSWORD, SALT)
    chacha_key, aes_key = crypto.split_v3_key(key64)

    scrypt_vector = {
        "profile": "fast",
        "password": PASSWORD,
        "salt_hex": SALT.hex(),
        "scrypt_n": crypto.SCRYPT_N,
        "scrypt_r": crypto.SCRYPT_R,
        "scrypt_p": crypto.SCRYPT_P,
        "key64_hex": key64.hex(),
        "chacha_key_hex": chacha_key.hex(),
        "aes_key_hex": aes_key.hex(),
    }
    (VECTOR_DIR / "scrypt_fast.json").write_text(
        json.dumps(scrypt_vector, indent=2) + "\n",
        encoding="utf-8",
    )

    c_nonce, a_nonce, meta_ct = crypto.encrypt_data_v3(key64, METADATA_JSON)
    metadata_vector = {
        "profile": "fast",
        "password": PASSWORD,
        "salt_hex": SALT.hex(),
        "plaintext_hex": METADATA_JSON.hex(),
        "chacha_nonce_hex": c_nonce.hex(),
        "aes_nonce_hex": a_nonce.hex(),
        "ciphertext_hex": meta_ct.hex(),
    }
    (VECTOR_DIR / "metadata_v3_fast.json").write_text(
        json.dumps(metadata_vector, indent=2) + "\n",
        encoding="utf-8",
    )

    _random_counter = 0
    stream_out = io.BytesIO()
    crypto.encrypt_stream_v5(key64, io.BytesIO(PLAINTEXT), stream_out, compress=True)
    stream_bytes = stream_out.getvalue()
    (VECTOR_DIR / "stream_v5_fast.bin").write_bytes(stream_bytes)

    stream_vector = {
        "profile": "fast",
        "password": PASSWORD,
        "salt_hex": SALT.hex(),
        "plaintext_hex": PLAINTEXT.hex(),
        "stream_hex": stream_bytes.hex(),
        "stream_size": len(stream_bytes),
    }
    (VECTOR_DIR / "stream_v5_fast.json").write_text(
        json.dumps(stream_vector, indent=2) + "\n",
        encoding="utf-8",
    )

    print(f"Wrote vectors to {VECTOR_DIR}")


if __name__ == "__main__":
    main()