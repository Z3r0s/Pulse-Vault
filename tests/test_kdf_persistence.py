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

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from pulsevault.core.vault import EncryptedVault, VaultError


class KdfPersistenceTests(unittest.TestCase):
    def setUp(self):
        self.root = Path.cwd() / f"pulse_kdf_{uuid.uuid4().hex}"
        self.root.mkdir()
        tempfile.tempdir = str(self.root)

    def tearDown(self):
        tempfile.tempdir = None
        shutil.rmtree(self.root, ignore_errors=True)

    def test_new_vault_writes_kdf_json(self):
        vault_path = self.root / "kdf-record.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("kdf-password-123!", scrypt_profile="standard")

        with zipfile.ZipFile(vault_path, "r") as z:
            record = json.loads(z.read("kdf.json").decode("utf-8"))

        self.assertEqual(record["algorithm"], "scrypt")
        self.assertEqual(record["profile"], "standard")
        self.assertEqual(record["n"], 32768)
        self.assertEqual(record["r"], 8)
        self.assertEqual(record["p"], 1)

    def test_unlock_uses_kdf_json_not_default_profile(self):
        vault_path = self.root / "override.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("override-password-123!", scrypt_profile="fast")

        with zipfile.ZipFile(vault_path, "r") as z:
            record = json.loads(z.read("kdf.json").decode("utf-8"))

        self.assertEqual(record["profile"], "fast")
        self.assertEqual(record["n"], 16)

        reopened = EncryptedVault(vault_path)
        reopened.unlock("override-password-123!")
        self.assertEqual(reopened.scrypt_profile, "fast")
        self.assertEqual(reopened.kdf_n, 16)

    def test_change_password_preserves_kdf_profile(self):
        vault_path = self.root / "rotate-kdf.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("rotate-old-password-123!", scrypt_profile="fast")

        before = json.loads(zipfile.ZipFile(vault_path, "r").read("kdf.json"))
        vault.change_password("rotate-old-password-123!", "rotate-new-password-456!")
        after = json.loads(zipfile.ZipFile(vault_path, "r").read("kdf.json"))

        self.assertEqual(before["profile"], "fast")
        self.assertEqual(after, before)

        reopened = EncryptedVault(vault_path)
        reopened.unlock("rotate-new-password-456!")
        self.assertEqual(reopened.scrypt_profile, "fast")

    def test_invalid_kdf_json_rejects_unlock(self):
        vault_path = self.root / "bad-kdf.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("bad-kdf-password-123!")

        with zipfile.ZipFile(vault_path, "r") as old_z:
            entries = {name: old_z.read(name) for name in old_z.namelist()}
        entries["kdf.json"] = b'{"algorithm":"pbkdf2"}'
        with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as new_z:
            for name, data in entries.items():
                new_z.writestr(name, data)

        with self.assertRaises(VaultError):
            EncryptedVault(vault_path).unlock("bad-kdf-password-123!")


if __name__ == "__main__":
    unittest.main()