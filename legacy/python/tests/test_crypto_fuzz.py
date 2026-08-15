import io
import os
import random
import sys
import unittest
from pathlib import Path

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

import pulsevault.core.crypto as crypto

try:
    from hypothesis import given, settings
    from hypothesis import strategies as st

    HAS_HYPOTHESIS = True
except ImportError:
    HAS_HYPOTHESIS = False


class CryptoFuzzTests(unittest.TestCase):
    def setUp(self):
        self.key = crypto.derive_key_v3("fuzz-password-123!", b"\x33" * crypto.SALT_SIZE)
        payload = io.BytesIO()
        crypto.encrypt_stream_v5(self.key, io.BytesIO(b"fuzz baseline payload" * 16), payload)
        self.stream_blob = payload.getvalue()

    def test_random_stream_truncations_raise_crypto_error(self):
        rng = random.Random(20260622)
        for _ in range(48):
            cut = rng.randint(1, max(1, len(self.stream_blob) - 1))
            with self.subTest(cut=cut):
                with self.assertRaises(crypto.CryptoError):
                    crypto.decrypt_stream_v5(self.key, io.BytesIO(self.stream_blob[:cut]), io.BytesIO())

    def test_random_single_byte_corruption_raises_crypto_error(self):
        rng = random.Random(20260623)
        for _ in range(24):
            offset = rng.randint(0, len(self.stream_blob) - 1)
            tampered = bytearray(self.stream_blob)
            tampered[offset] ^= 0x01
            with self.subTest(offset=offset):
                with self.assertRaises(crypto.CryptoError):
                    crypto.decrypt_stream_v5(self.key, io.BytesIO(bytes(tampered)), io.BytesIO())


if HAS_HYPOTHESIS:

    class CryptoHypothesisFuzzTests(unittest.TestCase):
        def setUp(self):
            self.key = crypto.derive_key_v3("fuzz-password-123!", b"\x33" * crypto.SALT_SIZE)
            payload = io.BytesIO()
            crypto.encrypt_stream_v5(self.key, io.BytesIO(b"fuzz baseline payload" * 16), payload)
            self.stream_blob = payload.getvalue()

        @given(st.integers(min_value=1, max_value=4096))
        @settings(max_examples=24, deadline=None)
        def test_hypothesis_stream_truncations(self, cut):
            bounded = min(cut, len(self.stream_blob) - 1)
            with self.assertRaises(crypto.CryptoError):
                crypto.decrypt_stream_v5(self.key, io.BytesIO(self.stream_blob[:bounded]), io.BytesIO())


if __name__ == "__main__":
    unittest.main()