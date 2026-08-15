"""Deep security / negative-path vault tests.

Restored/rewritten after disk corruption wiped the prior module. Covers wrong
passwords, metadata and stream tamper, ZIP store policy, and no-leak invariants
on the real EncryptedVault path.
"""

from __future__ import annotations

import json
import os
import shutil
import sys
import tempfile
import unittest
import uuid
import zipfile
from pathlib import Path

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")
os.environ.setdefault("PULSEVAULT_SCRYPT_PROFILE", "fast")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

import pulsevault.core.crypto as crypto
from pulsevault.core.vault import EncryptedVault, VaultError


class VaultSecurityDeepTests(unittest.TestCase):
    def setUp(self):
        self.root = Path.cwd() / f"pulse_sec_{uuid.uuid4().hex[:12]}"
        self.root.mkdir()
        tempfile.tempdir = str(self.root)
        self.password = "security-deep-password!!"

    def tearDown(self):
        tempfile.tempdir = None
        shutil.rmtree(self.root, ignore_errors=True)

    def _make_vault_with_file(self, name: str = "sec.pulsevault", payload: bytes = b"secret-bytes") -> Path:
        path = self.root / name
        v = EncryptedVault(path)
        v.create(self.password, scrypt_profile="fast")
        src = self.root / "payload.bin"
        src.write_bytes(payload)
        v.add_file(src, vault_name="payload.bin")
        v.lock()
        return path

    def test_wrong_password_dictionary_fails_cleanly(self):
        path = self._make_vault_with_file()
        bad = [
            "wrong",
            "security-deep-password!",
            "SECURITY-DEEP-PASSWORD!!",
            "security-deep-password!!!",
            "a" * 20,
        ]
        for pw in bad:
            locked = EncryptedVault(path)
            with self.assertRaises(VaultError):
                locked.unlock(pw)
            self.assertFalse(locked.is_unlocked)
        # Empty password is rejected by KDF before vault error wrapping
        locked = EncryptedVault(path)
        with self.assertRaises(Exception):
            locked.unlock("")
        self.assertFalse(locked.is_unlocked)

    def test_metadata_bitflip_prevents_unlock(self):
        path = self._make_vault_with_file("meta-tamper.pulsevault")
        with zipfile.ZipFile(path, "r") as zin:
            names = zin.namelist()
            members = {n: zin.read(n) for n in names}
        meta = bytearray(members["metadata.enc"])
        meta[len(meta) // 2] ^= 0x5A
        members["metadata.enc"] = bytes(meta)
        tmp = path.with_suffix(".tmp")
        with zipfile.ZipFile(tmp, "w", compression=zipfile.ZIP_STORED) as zout:
            for n, data in members.items():
                zout.writestr(n, data)
        tmp.replace(path)
        with self.assertRaises(VaultError):
            EncryptedVault(path).unlock(self.password)

    def test_blob_bitflip_fails_extract_and_verify(self):
        path = self._make_vault_with_file("blob-tamper.pulsevault", payload=b"tamper-me" * 100)
        with zipfile.ZipFile(path, "r") as zin:
            members = {n: zin.read(n) for n in zin.namelist()}
        blob_name = next(n for n in members if n.startswith("data/") and n.endswith(".enc"))
        blob = bytearray(members[blob_name])
        # Flip a late body byte (avoid pure header-only no-ops)
        blob[max(40, len(blob) // 2)] ^= 0xFF
        members[blob_name] = bytes(blob)
        tmp = path.with_suffix(".tmp")
        with zipfile.ZipFile(tmp, "w", compression=zipfile.ZIP_STORED) as zout:
            for n, data in members.items():
                zout.writestr(n, data)
        tmp.replace(path)

        v = EncryptedVault(path)
        v.unlock(self.password)
        with self.assertRaises(VaultError):
            v.verify_file("payload.bin")
        with self.assertRaises(VaultError):
            v.extract_file("payload.bin", self.root / "out", overwrite=True)

    def test_deflated_zip_member_rejected(self):
        path = self.root / "deflate.pulsevault"
        v = EncryptedVault(path)
        v.create(self.password, scrypt_profile="fast")
        v.lock()
        # Rebuild vault ZIP with DEFLATED metadata (policy violation)
        with zipfile.ZipFile(path, "r") as zin:
            members = {n: zin.read(n) for n in zin.namelist()}
        tmp = path.with_suffix(".tmp")
        with zipfile.ZipFile(tmp, "w", compression=zipfile.ZIP_DEFLATED) as zout:
            for n, data in members.items():
                zout.writestr(n, data)
        tmp.replace(path)
        with self.assertRaises(VaultError):
            EncryptedVault(path).unlock(self.password)

    def test_invalid_kdf_json_rejects_unlock(self):
        path = self._make_vault_with_file("bad-kdf.pulsevault")
        with zipfile.ZipFile(path, "r") as zin:
            members = {n: zin.read(n) for n in zin.namelist()}
        members["kdf.json"] = b'{"algorithm":"scrypt","profile":"fast","n":0,"r":8,"p":1}'
        tmp = path.with_suffix(".tmp")
        with zipfile.ZipFile(tmp, "w", compression=zipfile.ZIP_STORED) as zout:
            for n, data in members.items():
                zout.writestr(n, data)
        tmp.replace(path)
        with self.assertRaises(VaultError):
            EncryptedVault(path).unlock(self.password)

    def test_locked_vault_does_not_expose_key(self):
        path = self.root / "mem.pulsevault"
        v = EncryptedVault(path)
        v.create(self.password, scrypt_profile="fast")
        self.assertTrue(v.is_unlocked)
        self.assertFalse(hasattr(v, "password"))
        v.lock()
        self.assertFalse(v.is_unlocked)
        self.assertIsNone(getattr(v, "key", None) or None)

    def test_stream_wrong_key_does_not_leak_plaintext(self):
        key = crypto.derive_key_v3("correct-key-password!", b"\x11" * crypto.SALT_SIZE)
        wrong = crypto.derive_key_v3("incorrect-key-password", b"\x11" * crypto.SALT_SIZE)
        plain = b"highly-secret-content-do-not-leak"
        buf = __import__("io").BytesIO()
        crypto.encrypt_stream_v5(key, __import__("io").BytesIO(plain), buf, compress=True)
        blob = buf.getvalue()
        out = __import__("io").BytesIO()
        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_stream_v5(wrong, __import__("io").BytesIO(blob), out)
        leaked = out.getvalue()
        self.assertNotIn(b"highly-secret", leaked)

    def test_change_password_wrong_old_rejected(self):
        path = self.root / "chpw.pulsevault"
        v = EncryptedVault(path)
        v.create(self.password, scrypt_profile="fast")
        with self.assertRaises(VaultError):
            v.change_password("not-the-password!!!!", "another-password!!!!")
        # still unlocks with original
        v2 = EncryptedVault(path)
        v2.unlock(self.password)

    def test_missing_format_or_salt_structure(self):
        path = self.root / "broken.pulsevault"
        # Minimal invalid zip
        with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_STORED) as z:
            z.writestr("not-a-vault.txt", b"nope")
        with self.assertRaises(VaultError):
            EncryptedVault(path).unlock(self.password)


if __name__ == "__main__":
    unittest.main()
