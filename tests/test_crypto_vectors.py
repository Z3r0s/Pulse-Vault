import io
import json
import os
import sys
import unittest
from pathlib import Path

os.environ["PULSEVAULT_SCRYPT_PROFILE"] = "fast"

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

import pulsevault.core.crypto as crypto

VECTOR_DIR = Path(__file__).resolve().parent / "vectors"


class CryptoVectorTests(unittest.TestCase):
    def test_scrypt_fast_vector(self):
        vector = json.loads((VECTOR_DIR / "scrypt_fast.json").read_text(encoding="utf-8"))
        salt = bytes.fromhex(vector["salt_hex"])
        key64 = crypto.derive_key_v3(vector["password"], salt)

        self.assertEqual(crypto.SCRYPT_N, vector["scrypt_n"])
        self.assertEqual(crypto.SCRYPT_R, vector["scrypt_r"])
        self.assertEqual(crypto.SCRYPT_P, vector["scrypt_p"])
        self.assertEqual(key64.hex(), vector["key64_hex"])

        chacha_key, aes_key = crypto.split_v3_key(key64)
        self.assertEqual(chacha_key.hex(), vector["chacha_key_hex"])
        self.assertEqual(aes_key.hex(), vector["aes_key_hex"])

    def test_metadata_v3_fast_vector(self):
        vector = json.loads((VECTOR_DIR / "metadata_v3_fast.json").read_text(encoding="utf-8"))
        salt = bytes.fromhex(vector["salt_hex"])
        key64 = crypto.derive_key_v3(vector["password"], salt)
        plaintext = crypto.decrypt_data_v3(
            key64,
            bytes.fromhex(vector["chacha_nonce_hex"]),
            bytes.fromhex(vector["aes_nonce_hex"]),
            bytes.fromhex(vector["ciphertext_hex"]),
        )

        self.assertEqual(plaintext.hex(), vector["plaintext_hex"])

    def test_stream_v5_fast_vector_round_trip(self):
        vector = json.loads((VECTOR_DIR / "stream_v5_fast.json").read_text(encoding="utf-8"))
        stream_bytes = (VECTOR_DIR / "stream_v5_fast.bin").read_bytes()
        self.assertEqual(stream_bytes.hex(), vector["stream_hex"])

        salt = bytes.fromhex(vector["salt_hex"])
        key64 = crypto.derive_key_v3(vector["password"], salt)
        out = io.BytesIO()
        crypto.decrypt_stream_v5(key64, io.BytesIO(stream_bytes), out)

        self.assertEqual(out.getvalue().hex(), vector["plaintext_hex"])

    def test_scrypt_profiles_are_documented(self):
        self.assertEqual(set(crypto.SCRYPT_PROFILES), {"fast", "standard", "hardened"})
        self.assertGreater(crypto.SCRYPT_PROFILES["hardened"]["n"], crypto.SCRYPT_PROFILES["standard"]["n"])
        self.assertGreater(crypto.SCRYPT_PROFILES["standard"]["n"], crypto.SCRYPT_PROFILES["fast"]["n"])


if __name__ == "__main__":
    unittest.main()