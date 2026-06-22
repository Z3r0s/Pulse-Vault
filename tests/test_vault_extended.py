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
sys.path.insert(0, str(Path(__file__).resolve().parent))

import pulsevault.core.crypto as crypto
from pulsevault.core.vault import FORMAT_V5, EncryptedVault, VaultError
from vault_fixtures import (
    build_legacy_v1_vault,
    build_legacy_v2_vault,
    build_legacy_v3_vault,
    build_legacy_v4_vault,
)


class VaultExtendedTests(unittest.TestCase):
    def setUp(self):
        self.root = Path.cwd() / f"pulse_ext_{uuid.uuid4().hex}"
        self.root.mkdir()
        tempfile.tempdir = str(self.root)

    def tearDown(self):
        tempfile.tempdir = None
        shutil.rmtree(self.root, ignore_errors=True)

    def test_carrier_offset_preserved_without_carrier_path(self):
        carrier = self.root / "cover.png"
        carrier_payload = b"\x89PNG\r\n\x1a\n" + b"carrier" * 64
        carrier.write_bytes(carrier_payload)

        source = self.root / "secret.txt"
        source.write_bytes(b"carrier offset payload")
        vault_path = self.root / "offset-carrier.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("offset-password-123!", carrier_path=carrier)
        vault.add_file(source)
        del vault.carrier_path

        vault.add_file(source, overwrite=True)
        raw = vault_path.read_bytes()
        self.assertTrue(raw.startswith(carrier_payload))

    def test_carrier_file_round_trip(self):
        carrier = self.root / "cover.jpg"
        carrier_payload = b"\xff\xd8\xff\xe0fake-jpeg-prefix" + b"carrier-bytes" * 128
        carrier.write_bytes(carrier_payload)

        source = self.root / "hidden.txt"
        file_payload = b"hidden in carrier" * 16
        source.write_bytes(file_payload)

        vault_path = self.root / "carrier.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("carrier-password-123!", carrier_path=carrier)
        vault.add_file(source)

        raw = vault_path.read_bytes()
        self.assertTrue(raw.startswith(carrier_payload))
        self.assertGreater(len(raw), len(carrier_payload))

        reopened = EncryptedVault(vault_path)
        reopened.unlock("carrier-password-123!")
        extracted = reopened.extract_file("hidden.txt", self.root / "carrier-out")
        self.assertEqual(extracted.read_bytes(), file_payload)

        reopened.add_file(self.root / "carrier-out" / "hidden.txt", overwrite=True)
        raw_after = vault_path.read_bytes()
        self.assertTrue(raw_after.startswith(carrier_payload))

    def test_legacy_v1_vault_unlock_and_migrate(self):
        payload = b"v1 inline payload"
        vault_path = self.root / "legacy-v1.pulsevault"
        build_legacy_v1_vault(
            vault_path,
            "legacy-v1-password-123!",
            {"legacy-v1.txt": payload},
        )

        vault = EncryptedVault(vault_path)
        vault.unlock("legacy-v1-password-123!")
        self.assertEqual(vault.version, 1)

        extracted = vault.extract_file("legacy-v1.txt", self.root / "v1-out")
        self.assertEqual(extracted.read_bytes(), payload)

        vault.migrate_to_current_format("legacy-v1-password-123!")
        migrated = EncryptedVault(vault_path)
        migrated.unlock("legacy-v1-password-123!")
        extracted_again = migrated.extract_file("legacy-v1.txt", self.root / "v1-migrated")
        self.assertEqual(extracted_again.read_bytes(), payload)

    def test_legacy_v2_vault_unlock_and_migrate(self):
        payload = b"v2 blob payload" * 4
        vault_path = self.root / "legacy-v2.pulsevault"
        build_legacy_v2_vault(
            vault_path,
            "legacy-v2-password-123!",
            {"legacy-v2.txt": payload},
        )

        vault = EncryptedVault(vault_path)
        vault.unlock("legacy-v2-password-123!")
        self.assertEqual(vault.version, 2)

        extracted = vault.extract_file("legacy-v2.txt", self.root / "v2-out")
        self.assertEqual(extracted.read_bytes(), payload)

        vault.migrate_to_current_format("legacy-v2-password-123!")
        migrated = EncryptedVault(vault_path)
        migrated.unlock("legacy-v2-password-123!")
        extracted_again = migrated.extract_file("legacy-v2.txt", self.root / "v2-migrated")
        self.assertEqual(extracted_again.read_bytes(), payload)

    def test_legacy_v3_vault_unlock_and_migrate(self):
        payload = b"v3 legacy payload" * 8
        vault_path = self.root / "legacy-v3.pulsevault"
        build_legacy_v3_vault(
            vault_path,
            "legacy-v3-password-123!",
            {"legacy-v3.txt": payload},
        )

        vault = EncryptedVault(vault_path)
        vault.unlock("legacy-v3-password-123!")
        self.assertEqual(vault.version, 3)

        extracted = vault.extract_file("legacy-v3.txt", self.root / "v3-out")
        self.assertEqual(extracted.read_bytes(), payload)

        vault.migrate_to_current_format("legacy-v3-password-123!")
        self.assertEqual(vault.version, 5)
        with zipfile.ZipFile(vault_path, "r") as z:
            self.assertEqual(z.read("format.txt"), FORMAT_V5)

        migrated = EncryptedVault(vault_path)
        migrated.unlock("legacy-v3-password-123!")
        extracted_again = migrated.extract_file("legacy-v3.txt", self.root / "v3-migrated")
        self.assertEqual(extracted_again.read_bytes(), payload)

    def test_legacy_v4_vault_unlock_and_migrate(self):
        payload = b"v4 stream payload" * 16
        vault_path = self.root / "legacy-v4.pulsevault"
        build_legacy_v4_vault(
            vault_path,
            "legacy-v4-password-123!",
            {"legacy-v4.txt": payload},
        )

        vault = EncryptedVault(vault_path)
        vault.unlock("legacy-v4-password-123!")
        self.assertEqual(vault.version, 4)

        extracted = vault.extract_file("legacy-v4.txt", self.root / "v4-out")
        self.assertEqual(extracted.read_bytes(), payload)

        vault.migrate_to_current_format("legacy-v4-password-123!")
        self.assertEqual(vault.version, 5)

        migrated = EncryptedVault(vault_path)
        migrated.unlock("legacy-v4-password-123!")
        extracted_again = migrated.extract_file("legacy-v4.txt", self.root / "v4-migrated")
        self.assertEqual(extracted_again.read_bytes(), payload)

    def test_change_password_with_multiple_files(self):
        files = {
            "one.txt": b"first file payload",
            "two.txt": b"second file payload" * 64,
            "three.txt": b"third file payload" * 128,
        }
        vault_path = self.root / "multi-change.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("multi-old-password-123!")

        for name, payload in files.items():
            path = self.root / name
            path.write_bytes(payload)
            vault.add_file(path)

        vault.change_password("multi-old-password-123!", "multi-new-password-456!")

        with self.assertRaises(VaultError):
            EncryptedVault(vault_path).unlock("multi-old-password-123!")

        reopened = EncryptedVault(vault_path)
        reopened.unlock("multi-new-password-456!")
        for name, payload in files.items():
            extracted = reopened.extract_file(name, self.root / "multi-out")
            self.assertEqual(extracted.read_bytes(), payload)

        report = reopened.verify_all()
        self.assertEqual(report["file_count"], 3)

    def test_stream_decoder_rejects_truncated_headers(self):
        key = crypto.derive_key_v3("truncation-password!", b"\x11" * crypto.SALT_SIZE)
        encrypted = io.BytesIO()
        crypto.encrypt_stream_v5(key, io.BytesIO(b"truncation test"), encrypted)
        blob = encrypted.getvalue()

        for cut in (1, 8, 20, 30):
            with self.subTest(cut=cut):
                with self.assertRaises(crypto.CryptoError):
                    crypto.decrypt_stream_v5(key, io.BytesIO(blob[:cut]), io.BytesIO())

    def test_stream_decoder_rejects_truncated_chunks(self):
        key = crypto.derive_key_v3("chunk-trunc-password!", b"\x22" * crypto.SALT_SIZE)
        encrypted = io.BytesIO()
        crypto.encrypt_stream_v5(key, io.BytesIO(b"chunk truncation test" * 32), encrypted)
        blob = encrypted.getvalue()

        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_stream_v5(key, io.BytesIO(blob[:-4]), io.BytesIO())

        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_stream_v5(key, io.BytesIO(blob[:-1]), io.BytesIO())

    def test_zip_parser_rejects_truncated_vault(self):
        vault_path = self.root / "zip-trunc.pulsevault"
        vault = EncryptedVault(vault_path)
        vault.create("zip-trunc-password-123!")

        truncated = vault_path.read_bytes()[:64]
        broken_path = self.root / "broken.pulsevault"
        broken_path.write_bytes(truncated)

        with self.assertRaises(VaultError):
            EncryptedVault(broken_path).unlock("zip-trunc-password-123!")


if __name__ == "__main__":
    unittest.main()