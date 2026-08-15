"""Deep vault lifecycle tests (create → multi-add → verify → chpw → delete → lock).

Restored/rewritten after disk corruption wiped the prior module. Drives the real
EncryptedVault product path with PULSEVAULT_TEST_FAST_KDF for CI speed.
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

from pulsevault.core.vault import EncryptedVault, VaultError


class VaultLifecycleDeepTests(unittest.TestCase):
    def setUp(self):
        self.root = Path.cwd() / f"pulse_life_{uuid.uuid4().hex[:12]}"
        self.root.mkdir()
        tempfile.tempdir = str(self.root)

    def tearDown(self):
        tempfile.tempdir = None
        shutil.rmtree(self.root, ignore_errors=True)

    def _vault(self, name: str = "life.pulsevault") -> tuple[Path, EncryptedVault]:
        path = self.root / name
        v = EncryptedVault(path)
        return path, v

    def test_multi_file_lifecycle_verify_and_change_password(self):
        path, vault = self._vault()
        vault.create("life-old-password-123!", scrypt_profile="fast")
        payloads = {}
        for i, label in enumerate(("alpha.bin", "beta.txt", "gamma.dat")):
            p = self.root / label
            data = (f"payload-{label}-" * (8 + i)).encode() + bytes([i]) * 64
            p.write_bytes(data)
            payloads[label] = data
            vault.add_file(p)

        self.assertEqual(sorted(vault.list_files()), sorted(payloads))
        report = vault.verify_all()
        self.assertEqual(report["file_count"], 3)
        self.assertGreater(report["bytes_checked"], 0)
        self.assertEqual(report["hash_checked_count"], 3)

        for name, data in payloads.items():
            out = vault.extract_file(name, self.root / "out1", overwrite=True)
            self.assertEqual(out.read_bytes(), data)

        vault.change_password("life-old-password-123!", "life-new-password-456!")

        with self.assertRaises(VaultError):
            EncryptedVault(path).unlock("life-old-password-123!")

        reopened = EncryptedVault(path)
        reopened.unlock("life-new-password-456!")
        reopened.verify_all()
        for name, data in payloads.items():
            out = reopened.extract_file(name, self.root / "out2", overwrite=True)
            self.assertEqual(out.read_bytes(), data)

        reopened.delete_file("beta.txt")
        self.assertNotIn("beta.txt", reopened.list_files())
        reopened.verify_all()
        reopened.lock()
        self.assertFalse(reopened.is_unlocked)

    def test_kdf_json_persists_across_lifecycle(self):
        path, vault = self._vault("kdf-life.pulsevault")
        vault.create("kdf-life-password-123!", scrypt_profile="fast")
        with zipfile.ZipFile(path, "r") as z:
            rec = json.loads(z.read("kdf.json").decode("utf-8"))
        self.assertEqual(rec["algorithm"], "scrypt")
        self.assertEqual(rec["profile"], "fast")
        self.assertEqual(int(rec["n"]), 16)

        vault.change_password("kdf-life-password-123!", "kdf-life-password-999!")
        with zipfile.ZipFile(path, "r") as z:
            rec2 = json.loads(z.read("kdf.json").decode("utf-8"))
        self.assertEqual(int(rec2["n"]), 16)
        self.assertEqual(rec2["profile"], "fast")

        locked = EncryptedVault(path)
        locked.unlock("kdf-life-password-999!")
        self.assertEqual(locked.scrypt_profile, "fast")

    def test_overwrite_add_and_verify_hash_update(self):
        path, vault = self._vault("ow.pulsevault")
        vault.create("overwrite-password-123!", scrypt_profile="fast")
        f = self.root / "same.txt"
        f.write_bytes(b"first-version")
        vault.add_file(f)
        meta1 = vault.get_file_meta("same.txt")
        f.write_bytes(b"second-version-longer")
        vault.add_file(f, overwrite=True)
        meta2 = vault.get_file_meta("same.txt")
        self.assertNotEqual(meta1["sha256"], meta2["sha256"])
        vault.verify_file("same.txt")
        out = vault.extract_file("same.txt", self.root / "out", overwrite=True)
        self.assertEqual(out.read_bytes(), b"second-version-longer")

    def test_empty_vault_verify_and_stats(self):
        path, vault = self._vault("empty.pulsevault")
        vault.create("empty-vault-password!!", scrypt_profile="fast")
        report = vault.verify_all()
        self.assertEqual(report["file_count"], 0)
        stats = vault.stats()
        self.assertEqual(stats["file_count"], 0)
        self.assertEqual(stats["total_plain_size"], 0)

    def test_rename_then_verify_extract(self):
        path, vault = self._vault("rename.pulsevault")
        vault.create("rename-password-12345!", scrypt_profile="fast")
        f = self.root / "oldname.bin"
        data = b"rename-me" * 32
        f.write_bytes(data)
        vault.add_file(f)
        vault.rename_file("oldname.bin", "newname.bin")
        self.assertEqual(vault.list_files(), ["newname.bin"])
        vault.verify_file("newname.bin")
        out = vault.extract_file("newname.bin", self.root / "out", overwrite=True)
        self.assertEqual(out.read_bytes(), data)

    def test_sequential_password_rotations(self):
        path, vault = self._vault("rotate.pulsevault")
        pw0 = "rotate-password-000!"
        vault.create(pw0, scrypt_profile="fast")
        f = self.root / "x.bin"
        f.write_bytes(b"stable-payload")
        vault.add_file(f)
        passwords = [pw0, "rotate-password-001!", "rotate-password-002!", "rotate-password-003!"]
        for i in range(len(passwords) - 1):
            vault.change_password(passwords[i], passwords[i + 1])
            with self.assertRaises(VaultError):
                EncryptedVault(path).unlock(passwords[i])
            nxt = EncryptedVault(path)
            nxt.unlock(passwords[i + 1])
            vault = nxt
        out = vault.extract_file("x.bin", self.root / "out", overwrite=True)
        self.assertEqual(out.read_bytes(), b"stable-payload")


if __name__ == "__main__":
    unittest.main()
