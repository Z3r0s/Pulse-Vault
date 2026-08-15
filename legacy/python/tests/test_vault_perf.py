import hashlib
import os
import shutil
import sys
import tempfile
import unittest
import uuid
from pathlib import Path
from unittest import mock

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from pulsevault.core.vault import EncryptedVault


class VaultPerfTests(unittest.TestCase):
    def setUp(self):
        self.root = Path.cwd() / f"pulse_perf_{uuid.uuid4().hex}"
        self.root.mkdir()
        tempfile.tempdir = str(self.root)

    def tearDown(self):
        tempfile.tempdir = None
        shutil.rmtree(self.root, ignore_errors=True)

    def test_add_file_hashes_during_encryption(self):
        source = self.root / "single-pass.bin"
        payload = b"single-pass hashing" * 4096
        source.write_bytes(payload)
        expected = hashlib.sha256(payload).hexdigest()

        vault_path = self.root / "single-pass.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("single-pass-password-123!")
        with mock.patch("pulsevault.core.vault.stream_sha256") as mocked_hash:
            vault.add_file(source)
            mocked_hash.assert_not_called()

        self.assertEqual(vault.get_file_meta("single-pass.bin")["sha256"], expected)

    def test_list_files_is_cached_until_metadata_changes(self):
        vault_path = self.root / "cache.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("cache-password-123!")

        first = vault.list_files()
        second = vault.list_files()
        self.assertIs(first, second)

        source = self.root / "one.txt"
        source.write_bytes(b"one")
        vault.add_file(source)
        third = vault.list_files()
        self.assertIsNot(first, third)

    def test_peek_scrypt_profile_reads_kdf_json(self):
        vault_path = self.root / "peek.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("peek-password-123!", scrypt_profile="fast")
        self.assertEqual(EncryptedVault.peek_scrypt_profile(vault_path), "fast")


if __name__ == "__main__":
    unittest.main()