import hashlib
import io
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

import pulsevault.core.crypto as crypto
import pulsevault.core.vault as vault_module
from pulsevault.core.vault import EncryptedVault, VaultError, b64e
from pulsevault.gui.app import is_reasonable_password, password_policy_error


class VaultCoreTests(unittest.TestCase):
    def setUp(self):
        self.root = Path.cwd() / f"pulse_test_{uuid.uuid4().hex}"
        self.root.mkdir()
        tempfile.tempdir = str(self.root)

    def tearDown(self):
        tempfile.tempdir = None
        shutil.rmtree(self.root, ignore_errors=True)

    def test_create_add_extract_and_change_password(self):
        source = self.root / "alpha.txt"
        payload = b"alpha secret data" * 64
        source.write_bytes(payload)

        vault_path = self.root / "test.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("old-password-123!")
        vault.add_file(source)

        extracted = vault.extract_file("alpha.txt", self.root / "out1")
        self.assertEqual(extracted.read_bytes(), payload)

        vault.change_password("old-password-123!", "new-password-456!")

        old_attempt = EncryptedVault(vault_path)
        with self.assertRaises(VaultError):
            old_attempt.unlock("old-password-123!")

        reopened = EncryptedVault(vault_path)
        reopened.unlock("new-password-456!")
        extracted_again = reopened.extract_file("alpha.txt", self.root / "out2")
        self.assertEqual(extracted_again.read_bytes(), payload)

        report = reopened.verify_all()
        self.assertEqual(report["file_count"], 1)
        self.assertEqual(report["bytes_checked"], len(payload))
        self.assertEqual(report["hash_checked_count"], 1)

    def test_vault_does_not_keep_plaintext_password_attribute(self):
        vault_path = self.root / "memory-check.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("memory-safe-password!")

        self.assertFalse(hasattr(vault, "password"))
        reopened = EncryptedVault(vault_path)
        reopened.unlock("memory-safe-password!")
        self.assertFalse(hasattr(reopened, "password"))

    def test_v5_stream_rejects_tampered_ciphertext(self):
        key = crypto.derive_key_v3("stream-password-123", b"0" * crypto.SALT_SIZE)
        encrypted = io.BytesIO()
        crypto.encrypt_stream_v5(key, io.BytesIO(b"payload"), encrypted)

        tampered = bytearray(encrypted.getvalue())
        tampered[-1] ^= 0x01

        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_stream_v5(key, io.BytesIO(tampered), io.BytesIO())

    def test_locked_vault_does_not_leak_filename_or_content(self):
        source = self.root / "very-secret-name.txt"
        payload = b"top secret payload that should not appear in the vault"
        source.write_bytes(payload)

        vault_path = self.root / "leak-check.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("leak-password-123!")
        vault.add_file(source)

        raw = vault_path.read_bytes()
        self.assertNotIn(b"very-secret-name.txt", raw)
        self.assertNotIn(payload, raw)

        with zipfile.ZipFile(vault_path, "r") as z:
            self.assertNotIn("very-secret-name.txt", z.namelist())
            self.assertTrue(any(name.startswith("data/") and name.endswith(".enc") for name in z.namelist()))

    def test_wrong_password_fails(self):
        vault_path = self.root / "wrong-password.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("correct-password-123!")

        locked = EncryptedVault(vault_path)
        with self.assertRaises(VaultError):
            locked.unlock("wrong-password-123!")

    def test_metadata_tamper_fails_unlock(self):
        vault_path = self.root / "metadata-tamper.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("metadata-password-123!")

        with zipfile.ZipFile(vault_path, "r") as old_z:
            entries = {name: old_z.read(name) for name in old_z.namelist()}
        metadata = bytearray(entries["metadata.enc"])
        metadata[-1] ^= 0x01
        entries["metadata.enc"] = bytes(metadata)
        with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as new_z:
            for name, data in entries.items():
                new_z.writestr(name, data)

        locked = EncryptedVault(vault_path)
        with self.assertRaises(VaultError):
            locked.unlock("metadata-password-123!")

    def test_compressed_zip_metadata_is_rejected_before_unlock(self):
        vault_path = self.root / "compressed-metadata.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("compressed-password-123!")

        with zipfile.ZipFile(vault_path, "r") as old_z:
            entries = {name: old_z.read(name) for name in old_z.namelist()}
        with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_DEFLATED) as new_z:
            for name, data in entries.items():
                new_z.writestr(name, data)

        locked = EncryptedVault(vault_path)
        with self.assertRaises(VaultError):
            locked.unlock("compressed-password-123!")

    def test_invalid_salt_size_is_rejected_before_unlock(self):
        vault_path = self.root / "bad-salt.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("salt-password-123!")

        with zipfile.ZipFile(vault_path, "r") as old_z:
            entries = {name: old_z.read(name) for name in old_z.namelist()}
        entries["salt.bin"] = b"short"
        with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as new_z:
            for name, data in entries.items():
                new_z.writestr(name, data)

        locked = EncryptedVault(vault_path)
        with self.assertRaises(VaultError):
            locked.unlock("salt-password-123!")

    def test_missing_blob_fails_verify(self):
        source = self.root / "missing.txt"
        source.write_bytes(b"missing blob payload")

        vault_path = self.root / "missing-blob.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("missing-password-123!")
        vault.add_file(source)

        with zipfile.ZipFile(vault_path, "r") as old_z:
            kept_entries = {
                name: old_z.read(name)
                for name in old_z.namelist()
                if not name.startswith("data/")
            }

        with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as new_z:
            for name, data in kept_entries.items():
                new_z.writestr(name, data)

        reopened = EncryptedVault(vault_path)
        reopened.unlock("missing-password-123!")
        with self.assertRaises(VaultError):
            reopened.verify_all()

    def test_failed_add_file_keeps_existing_vault_consistent(self):
        alpha = self.root / "alpha.txt"
        alpha_payload = b"alpha payload"
        alpha.write_bytes(alpha_payload)
        beta = self.root / "beta.txt"
        beta.write_bytes(b"beta payload")

        vault_path = self.root / "atomic-add.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("atomic-password-123!")
        vault.add_file(alpha)

        original_encrypt = vault_module.encrypt_stream_v5

        def failing_encrypt(*args, **kwargs):
            raise RuntimeError("simulated encryption failure")

        vault_module.encrypt_stream_v5 = failing_encrypt
        try:
            with self.assertRaises(RuntimeError):
                vault.add_file(beta)
        finally:
            vault_module.encrypt_stream_v5 = original_encrypt

        self.assertEqual(vault.list_files(), ["alpha.txt"])
        self.assertNotIn("beta.txt", vault.data["files"])

        reopened = EncryptedVault(vault_path)
        reopened.unlock("atomic-password-123!")
        self.assertEqual(reopened.list_files(), ["alpha.txt"])
        extracted = reopened.extract_file("alpha.txt", self.root / "atomic-out")
        self.assertEqual(extracted.read_bytes(), alpha_payload)

    def test_tampered_extract_removes_partial_and_preserves_existing_file(self):
        source = self.root / "alpha.txt"
        source.write_bytes(b"authenticated payload" * 256)

        vault_path = self.root / "tampered-extract.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("tamper-password-123!")
        vault.add_file(source)

        with zipfile.ZipFile(vault_path, "r") as old_z:
            entries = {name: old_z.read(name) for name in old_z.namelist()}
        data_name = next(name for name in entries if name.startswith("data/"))
        tampered = bytearray(entries[data_name])
        tampered[-1] ^= 0x01
        entries[data_name] = bytes(tampered)
        with zipfile.ZipFile(vault_path, "w", zipfile.ZIP_STORED) as new_z:
            for name, data in entries.items():
                new_z.writestr(name, data)

        out_dir = self.root / "tamper-out"
        out_dir.mkdir()
        existing = out_dir / "alpha.txt"
        existing.write_bytes(b"keep this existing file")

        with self.assertRaises(VaultError):
            vault.extract_file("alpha.txt", out_dir, overwrite=True)

        self.assertEqual(existing.read_bytes(), b"keep this existing file")
        self.assertFalse(any(path.suffix == ".part" for path in out_dir.iterdir()))

    def test_extract_refuses_symlink_output(self):
        source = self.root / "alpha.txt"
        source.write_bytes(b"symlink payload")

        vault_path = self.root / "symlink-output.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("symlink-password-123!")
        vault.add_file(source)

        out_dir = self.root / "symlink-out"
        out_dir.mkdir()
        target = self.root / "outside.txt"
        target.write_text("outside")
        link = out_dir / "alpha.txt"
        try:
            os.symlink(target, link)
        except (OSError, NotImplementedError):
            self.skipTest("symlink creation is not available in this environment")

        with self.assertRaises(VaultError):
            vault.extract_file("alpha.txt", out_dir, overwrite=True)
        self.assertEqual(target.read_text(), "outside")

    def test_extract_sanitizes_legacy_or_malicious_names(self):
        source = self.root / "alpha.txt"
        payload = b"path traversal payload"
        source.write_bytes(payload)

        vault_path = self.root / "safe-extract.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("safe-extract-password-123!")
        vault.add_file(source)
        vault.data["files"]["../evil.txt"] = vault.data["files"].pop("alpha.txt")

        out_dir = self.root / "safe-out"
        extracted = vault.extract_file("../evil.txt", out_dir)

        self.assertEqual(extracted, out_dir / ".._evil.txt")
        self.assertEqual(extracted.read_bytes(), payload)
        self.assertFalse((self.root / "evil.txt").exists())

    def test_folder_import_limits_file_count(self):
        folder = self.root / "folder"
        folder.mkdir()
        (folder / "one.txt").write_text("one")
        (folder / "two.txt").write_text("two")

        vault_path = self.root / "folder-limit.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("folder-password-123!")

        with self.assertRaises(VaultError):
            vault.add_folder_as_zip(folder, max_files=1)
        self.assertEqual(vault.list_files(), [])

    def test_file_hash_is_recorded_for_added_file(self):
        source = self.root / "hash-me.bin"
        payload = b"hash me" * 4096
        source.write_bytes(payload)

        vault_path = self.root / "hash-check.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("hash-password-123!")
        vault.add_file(source)

        meta = vault.get_file_meta("hash-me.bin")
        self.assertEqual(meta["sha256"], hashlib.sha256(payload).hexdigest())
        self.assertNotEqual(meta["sha256"], "skipped_large_file")

    def test_password_policy_rejects_common_or_repetitive_passwords(self):
        self.assertFalse(is_reasonable_password("password123456"))
        self.assertFalse(is_reasonable_password("aaaaaaaaaaaaaa"))
        self.assertIsNotNone(password_policy_error("qwerty123456789"))
        self.assertTrue(is_reasonable_password("Correct-Horse-72-Sunset"))

    def test_legacy_inline_content_is_saved_as_v5_stream(self):
        vault_path = self.root / "legacy-upgrade.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("upgrade-password-123!")
        vault.data["files"]["legacy.txt"] = {
            "name": "legacy.txt",
            "size": 13,
            "sha256": "skipped_large_file",
            "added_at": 1,
            "updated_at": 1,
            "type": "file",
            "content": b64e(b"legacy bytes!"),
        }
        vault.save()

        reopened = EncryptedVault(vault_path)
        reopened.unlock("upgrade-password-123!")
        out = reopened.extract_file("legacy.txt", self.root / "legacy-out")
        self.assertEqual(out.read_bytes(), b"legacy bytes!")


if __name__ == "__main__":
    unittest.main()
