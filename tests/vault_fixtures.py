import json
import time
import uuid
import zipfile
from pathlib import Path

from pulsevault.core.crypto import (
    SALT_SIZE,
    derive_key_v3,
    encrypt_data_v3,
    encrypt_stream_v4,
)
from pulsevault.core.vault import BytesReader, FORMAT_V3, FORMAT_V4, b64e


def build_legacy_v3_vault(
    vault_path: Path,
    password: str,
    files: dict[str, bytes],
    salt: bytes | None = None,
) -> None:
    salt = salt or (b"\xab" * SALT_SIZE)
    key = derive_key_v3(password, salt)
    now = int(time.time())
    entries: dict[str, dict] = {}

    with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as z:
        z.writestr("salt.bin", salt)
        z.writestr("format.txt", FORMAT_V3)

        for name, payload in files.items():
            internal_id = str(uuid.uuid4())
            c_nonce, a_nonce, ciphertext = encrypt_data_v3(key, payload)
            z.writestr(f"data/{internal_id}.enc", c_nonce + a_nonce + ciphertext)
            entries[name] = {
                "name": name,
                "size": len(payload),
                "sha256": "skipped_large_file",
                "added_at": now,
                "updated_at": now,
                "type": "file",
                "internal_id": internal_id,
            }

        metadata = {
            "version": 3,
            "created_at": now,
            "updated_at": now,
            "files": entries,
        }
        plaintext = json.dumps(metadata, indent=2).encode("utf-8")
        c_nonce, a_nonce, ciphertext = encrypt_data_v3(key, plaintext)
        z.writestr("metadata.enc", c_nonce + a_nonce + ciphertext)


def build_legacy_v4_vault(
    vault_path: Path,
    password: str,
    files: dict[str, bytes],
    salt: bytes | None = None,
) -> None:
    salt = salt or (b"\xcd" * SALT_SIZE)
    key = derive_key_v3(password, salt)
    now = int(time.time())
    entries: dict[str, dict] = {}

    with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as z:
        z.writestr("salt.bin", salt)
        z.writestr("format.txt", FORMAT_V4)

        for name, payload in files.items():
            internal_id = str(uuid.uuid4())
            with z.open(f"data/{internal_id}.enc", "w") as target:
                encrypt_stream_v4(key, BytesReader(payload), target)
            entries[name] = {
                "name": name,
                "size": len(payload),
                "sha256": "skipped_large_file",
                "added_at": now,
                "updated_at": now,
                "type": "file",
                "internal_id": internal_id,
            }

        metadata = {
            "version": 4,
            "created_at": now,
            "updated_at": now,
            "files": entries,
        }
        plaintext = json.dumps(metadata, indent=2).encode("utf-8")
        c_nonce, a_nonce, ciphertext = encrypt_data_v3(key, plaintext)
        z.writestr("metadata.enc", c_nonce + a_nonce + ciphertext)


def build_legacy_v3_inline_vault(
    vault_path: Path,
    password: str,
    filename: str,
    payload: bytes,
    salt: bytes | None = None,
) -> None:
    salt = salt or (b"\xef" * SALT_SIZE)
    key = derive_key_v3(password, salt)
    now = int(time.time())

    metadata = {
        "version": 3,
        "created_at": now,
        "updated_at": now,
        "files": {
            filename: {
                "name": filename,
                "size": len(payload),
                "sha256": "skipped_large_file",
                "added_at": now,
                "updated_at": now,
                "type": "file",
                "content": b64e(payload),
            }
        },
    }

    with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as z:
        z.writestr("salt.bin", salt)
        z.writestr("format.txt", FORMAT_V3)
        plaintext = json.dumps(metadata, indent=2).encode("utf-8")
        c_nonce, a_nonce, ciphertext = encrypt_data_v3(key, plaintext)
        z.writestr("metadata.enc", c_nonce + a_nonce + ciphertext)