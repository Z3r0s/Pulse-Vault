"""Deep crypto unit tests for Pulse-Vault.

Covers multi-chunk streaming, compression flags, AAD/MAC binding, KDF records,
derive_key_scrypt determinism, v3/v4/legacy-v5 paths, and helper sanitizers.

CI-safe: PULSEVAULT_TEST_FAST_KDF + small synthetic CHUNK_SIZE for multi-chunk.
"""

import hashlib
import io
import lzma
import os
import struct
import sys
import unittest
from pathlib import Path
from unittest import mock

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

import pulsevault.core.crypto as crypto

try:
    from hypothesis import given, settings
    from hypothesis import strategies as st

    HAS_HYPOTHESIS = True
except ImportError:
    HAS_HYPOTHESIS = False

# Small synthetic chunk size so multi-chunk paths run in CI without multi-MB payloads.
SMALL_CHUNK = 64


def _make_key(password: str = "deep-test-password-123!", salt: bytes | None = None) -> bytes:
    if salt is None:
        salt = b"\xab" * crypto.SALT_SIZE
    return crypto.derive_key_v3(password, salt)


def _encrypt_v5(key: bytes, plaintext: bytes, compress: bool = True, chunk_size: int = SMALL_CHUNK) -> bytes:
    out = io.BytesIO()
    with mock.patch.object(crypto, "CHUNK_SIZE", chunk_size):
        crypto.encrypt_stream_v5(key, io.BytesIO(plaintext), out, compress=compress)
    return out.getvalue()


def _decrypt_v5(key: bytes, blob: bytes) -> bytes:
    out = io.BytesIO()
    crypto.decrypt_stream_v5(key, io.BytesIO(blob), out)
    return out.getvalue()


def _build_legacy_v5_stream(key64: bytes, plaintext: bytes, compress: bool = False) -> bytes:
    """Construct a pre-magic V5 stream (no STREAM_V5_MAGIC, AAD=None).

    Layout expected by _decrypt_legacy_stream_v5:
        flag(1) | chacha_nonce(16) | aes_nonce(12) | repeated(chunk_len(4)|ct)
    """
    chacha_key, aes_key = crypto.split_v3_key(key64)
    chacha_nonce = os.urandom(16)
    aes_nonce = os.urandom(crypto.NONCE_SIZE)
    flag = b"\x01" if compress else b"\x00"

    chacha = crypto.ChaCha20Poly1305(chacha_key) if hasattr(crypto, "ChaCha20Poly1305") else None
    # Import AEAD classes the same way production code does
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM, ChaCha20Poly1305

    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)

    parts = [flag, chacha_nonce, aes_nonce]
    payload = plaintext
    if compress:
        payload = lzma.compress(plaintext, format=lzma.FORMAT_XZ, preset=1)

    # Single chunk is enough for legacy path coverage; multi-chunk optional.
    chunk_index = 0
    offset = 0
    while offset < len(payload) or (offset == 0 and not payload):
        piece = payload[offset : offset + SMALL_CHUNK]
        if not piece and offset > 0:
            break
        if not piece and not payload:
            break
        if not piece:
            break
        inner_ct = chacha.encrypt(crypto._chunk_nonce(chacha_nonce, chunk_index), piece, None)
        outer_ct = aesgcm.encrypt(crypto._chunk_nonce(aes_nonce, chunk_index), inner_ct, None)
        parts.append(len(outer_ct).to_bytes(4, byteorder="big"))
        parts.append(outer_ct)
        chunk_index += 1
        offset += len(piece)
        if not payload:
            break

    # Empty plaintext: still a valid header-only legacy stream
    return b"".join(parts)


class StreamV5MultiChunkTests(unittest.TestCase):
    def setUp(self):
        self.key = _make_key()

    def test_multi_chunk_roundtrip_uncompressed(self):
        # 3+ full chunks + remainder with SMALL_CHUNK
        plaintext = b"ABCDEFGHIJKLMNOPQRSTUVWXYZ012345" * 8  # 256 bytes → 4 chunks of 64
        self.assertGreater(len(plaintext), SMALL_CHUNK * 2)

        blob = _encrypt_v5(self.key, plaintext, compress=False, chunk_size=SMALL_CHUNK)
        recovered = _decrypt_v5(self.key, blob)
        self.assertEqual(recovered, plaintext)

    def test_exact_chunk_boundary_uncompressed(self):
        plaintext = b"x" * (SMALL_CHUNK * 3)  # exact multiple
        blob = _encrypt_v5(self.key, plaintext, compress=False, chunk_size=SMALL_CHUNK)
        self.assertEqual(_decrypt_v5(self.key, blob), plaintext)

    def test_single_byte_over_chunk_boundary(self):
        plaintext = b"y" * (SMALL_CHUNK + 1)
        blob = _encrypt_v5(self.key, plaintext, compress=False, chunk_size=SMALL_CHUNK)
        self.assertEqual(_decrypt_v5(self.key, blob), plaintext)

    def test_empty_plaintext_roundtrip(self):
        for compress in (True, False):
            with self.subTest(compress=compress):
                blob = _encrypt_v5(self.key, b"", compress=compress, chunk_size=SMALL_CHUNK)
                self.assertEqual(_decrypt_v5(self.key, blob), b"")

    def test_multi_chunk_roundtrip_compressed(self):
        # Highly compressible so LZMA still emits multi-chunk encrypted frames
        plaintext = (b"PulseVault multi-chunk compress payload " * 200)
        blob = _encrypt_v5(self.key, plaintext, compress=True, chunk_size=SMALL_CHUNK)
        self.assertEqual(_decrypt_v5(self.key, blob), plaintext)

    def test_chunk_boundary_sizes_matrix(self):
        sizes = [0, 1, SMALL_CHUNK - 1, SMALL_CHUNK, SMALL_CHUNK + 1, SMALL_CHUNK * 2, SMALL_CHUNK * 2 + 7]
        for size in sizes:
            for compress in (False, True):
                with self.subTest(size=size, compress=compress):
                    plaintext = bytes((i * 17 + size) % 256 for i in range(size))
                    blob = _encrypt_v5(self.key, plaintext, compress=compress, chunk_size=SMALL_CHUNK)
                    self.assertEqual(_decrypt_v5(self.key, blob), plaintext)


class StreamV5CompressFlagTests(unittest.TestCase):
    def setUp(self):
        self.key = _make_key("compress-flag-pw!")
        self.payload = b"identity payload for compress flag tests " * 12

    def test_compress_true_roundtrip(self):
        blob = _encrypt_v5(self.key, self.payload, compress=True)
        self.assertEqual(_decrypt_v5(self.key, blob), self.payload)

    def test_compress_false_roundtrip(self):
        blob = _encrypt_v5(self.key, self.payload, compress=False)
        self.assertEqual(_decrypt_v5(self.key, blob), self.payload)

    def test_compress_true_and_false_differ_on_wire(self):
        blob_c = _encrypt_v5(self.key, self.payload, compress=True)
        blob_u = _encrypt_v5(self.key, self.payload, compress=False)
        # Magic + flag differ at least in flag byte; wire layout should not match.
        self.assertEqual(blob_c[:8], crypto.STREAM_V5_MAGIC)
        self.assertEqual(blob_u[:8], crypto.STREAM_V5_MAGIC)
        self.assertEqual(blob_c[8], 0x01)
        self.assertEqual(blob_u[8], 0x00)
        # Both decrypt to identical plaintext
        self.assertEqual(_decrypt_v5(self.key, blob_c), self.payload)
        self.assertEqual(_decrypt_v5(self.key, blob_u), self.payload)

    def test_incompressible_data_still_roundtrips_with_compress(self):
        # Random-looking data exercises LZMA without relying on size reduction
        plaintext = bytes(range(256)) * 4
        blob = _encrypt_v5(self.key, plaintext, compress=True)
        self.assertEqual(_decrypt_v5(self.key, blob), plaintext)


class StreamV5IntegrityTests(unittest.TestCase):
    def setUp(self):
        self.key = _make_key("integrity-pw-deep!")
        self.plaintext = b"AAD binding and bitflip integrity payload" * 20
        self.blob = _encrypt_v5(self.key, self.plaintext, compress=False, chunk_size=SMALL_CHUNK)

    def test_wrong_key_fails(self):
        wrong = _make_key("wrong-password!!!!!", salt=b"\xcd" * crypto.SALT_SIZE)
        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(wrong, self.blob)

    def test_wrong_key_same_salt_fails(self):
        wrong = _make_key("almost-correct-pw!!", salt=b"\xab" * crypto.SALT_SIZE)
        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(wrong, self.blob)

    def test_bitflip_in_ciphertext_body_fails(self):
        # Flip a byte after header (magic 8 + flag 1 + chacha 16 + aes 12 = 37)
        header_len = len(crypto.STREAM_V5_MAGIC) + 1 + 16 + crypto.NONCE_SIZE
        self.assertGreater(len(self.blob), header_len + 8)
        for offset in (header_len + 4, header_len + 10, len(self.blob) - 1):
            tampered = bytearray(self.blob)
            tampered[offset] ^= 0xFF
            with self.subTest(offset=offset):
                with self.assertRaises(crypto.CryptoError):
                    _decrypt_v5(self.key, bytes(tampered))

    def test_bitflip_in_flag_fails(self):
        # flag is at index 8; flip compress flag 0x00 -> 0x01 breaks AAD binding
        tampered = bytearray(self.blob)
        flag_idx = len(crypto.STREAM_V5_MAGIC)
        tampered[flag_idx] ^= 0x01
        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(self.key, bytes(tampered))

    def test_bitflip_in_nonce_fails(self):
        # chacha nonce starts at index 9
        tampered = bytearray(self.blob)
        tampered[len(crypto.STREAM_V5_MAGIC) + 1] ^= 0x80
        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(self.key, bytes(tampered))

    def test_chunk_index_aad_binding_prevents_reorder(self):
        # With multi-chunk ciphertext, swapping two encrypted chunks must fail MAC/AAD
        plaintext = b"Z" * (SMALL_CHUNK * 3)
        blob = _encrypt_v5(self.key, plaintext, compress=False, chunk_size=SMALL_CHUNK)
        header_len = len(crypto.STREAM_V5_MAGIC) + 1 + 16 + crypto.NONCE_SIZE
        body = blob[header_len:]
        # Parse chunks
        chunks = []
        pos = 0
        while pos < len(body):
            clen = int.from_bytes(body[pos : pos + 4], "big")
            pos += 4
            chunks.append(body[pos : pos + clen])
            pos += clen
        self.assertGreaterEqual(len(chunks), 2)

        # Swap first two encrypted frames
        reordered = bytearray(blob[:header_len])
        order = [1, 0] + list(range(2, len(chunks)))
        for i in order:
            ct = chunks[i]
            reordered.extend(len(ct).to_bytes(4, "big"))
            reordered.extend(ct)

        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(self.key, bytes(reordered))


class ParseKdfRecordTests(unittest.TestCase):
    def test_valid_fast_record(self):
        record = crypto.kdf_record_from_profile("fast")
        profile, n, r, p = crypto.parse_kdf_record(record)
        self.assertEqual(profile, "fast")
        self.assertEqual((n, r, p), crypto.scrypt_params_for_profile("fast"))

    def test_valid_standard_record(self):
        record = {
            "algorithm": crypto.KDF_ALGORITHM_SCRYPT,
            "profile": "standard",
            "n": 2**15,
            "r": 8,
            "p": 1,
        }
        profile, n, r, p = crypto.parse_kdf_record(record)
        self.assertEqual(profile, "standard")
        self.assertEqual((n, r, p), (2**15, 8, 1))

    def test_missing_n_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "scrypt", "profile": "fast", "r": 8, "p": 1})

    def test_missing_r_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "scrypt", "profile": "fast", "n": 16, "p": 1})

    def test_missing_p_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "scrypt", "profile": "fast", "n": 16, "r": 8})

    def test_wrong_algorithm_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "pbkdf2", "profile": "fast", "n": 16, "r": 8, "p": 1})

    def test_missing_algorithm_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"profile": "fast", "n": 16, "r": 8, "p": 1})

    def test_invalid_n_zero_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "scrypt", "profile": "fast", "n": 0, "r": 8, "p": 1})

    def test_invalid_r_negative_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "scrypt", "profile": "fast", "n": 16, "r": -1, "p": 1})

    def test_invalid_p_zero_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record({"algorithm": "scrypt", "profile": "fast", "n": 16, "r": 8, "p": 0})

    def test_wrong_types_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record(
                {"algorithm": "scrypt", "profile": "fast", "n": "not-int", "r": 8, "p": 1}
            )
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record(
                {"algorithm": "scrypt", "profile": "fast", "n": None, "r": 8, "p": 1}
            )

    def test_unknown_profile_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.parse_kdf_record(
                {"algorithm": "scrypt", "profile": "ludicrous", "n": 16, "r": 8, "p": 1}
            )

    def test_default_profile_when_missing(self):
        # Missing profile defaults to "standard" then validates membership
        profile, n, r, p = crypto.parse_kdf_record(
            {"algorithm": "scrypt", "n": 2**15, "r": 8, "p": 1}
        )
        self.assertEqual(profile, "standard")
        self.assertEqual((n, r, p), (2**15, 8, 1))

    def test_string_numeric_params_coerced(self):
        profile, n, r, p = crypto.parse_kdf_record(
            {"algorithm": "scrypt", "profile": "fast", "n": "16", "r": "8", "p": "1"}
        )
        self.assertEqual(profile, "fast")
        self.assertEqual((n, r, p), (16, 8, 1))


class KdfProfileHelpersTests(unittest.TestCase):
    def test_kdf_record_from_profile_fast(self):
        rec = crypto.kdf_record_from_profile("fast")
        self.assertEqual(rec["algorithm"], crypto.KDF_ALGORITHM_SCRYPT)
        self.assertEqual(rec["profile"], "fast")
        n, r, p = crypto.scrypt_params_for_profile("fast")
        self.assertEqual(rec["n"], n)
        self.assertEqual(rec["r"], r)
        self.assertEqual(rec["p"], p)

    def test_kdf_record_from_profile_standard(self):
        rec = crypto.kdf_record_from_profile("standard")
        self.assertEqual(rec["profile"], "standard")
        n, r, p = crypto.scrypt_params_for_profile("standard")
        self.assertEqual((rec["n"], rec["r"], rec["p"]), (n, r, p))
        self.assertGreater(rec["n"], crypto.SCRYPT_PROFILES["fast"]["n"])

    def test_scrypt_params_for_profile_hardened(self):
        n, r, p = crypto.scrypt_params_for_profile("hardened")
        self.assertEqual((r, p), (8, 1))
        self.assertEqual(n, 2**20)

    def test_unknown_profile_rejected(self):
        with self.assertRaises(ValueError):
            crypto.scrypt_params_for_profile("nope")
        with self.assertRaises(ValueError):
            crypto.kdf_record_from_profile("nope")

    def test_empty_profile_rejected(self):
        with self.assertRaises(ValueError):
            crypto.scrypt_params_for_profile("")


class DeriveKeyScryptTests(unittest.TestCase):
    def test_determinism(self):
        salt = b"\x11" * crypto.SALT_SIZE
        k1 = crypto.derive_key_scrypt("same-password", salt, 16, 8, 1)
        k2 = crypto.derive_key_scrypt("same-password", salt, 16, 8, 1)
        self.assertEqual(k1, k2)
        self.assertEqual(len(k1), crypto.V3_KEY_SIZE)

    def test_different_salts_different_keys(self):
        k1 = crypto.derive_key_scrypt("pw", b"\x00" * 16, 16, 8, 1)
        k2 = crypto.derive_key_scrypt("pw", b"\xff" * 16, 16, 8, 1)
        self.assertNotEqual(k1, k2)

    def test_different_passwords_different_keys(self):
        salt = b"\x22" * 16
        k1 = crypto.derive_key_scrypt("password-a", salt, 16, 8, 1)
        k2 = crypto.derive_key_scrypt("password-b", salt, 16, 8, 1)
        self.assertNotEqual(k1, k2)

    def test_empty_password_raises(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.derive_key_scrypt("", b"\x00" * 16, 16, 8, 1)

    def test_matches_derive_key_v3_with_explicit_params(self):
        salt = b"\x33" * 16
        a = crypto.derive_key_scrypt("align", salt, 16, 8, 1)
        b = crypto.derive_key_v3("align", salt, n=16, r=8, p=1)
        self.assertEqual(a, b)


class EncryptDataV3Tests(unittest.TestCase):
    def setUp(self):
        self.key = _make_key("v3-meta-password!")

    def test_roundtrip(self):
        plaintext = b'{"files": [], "version": 5}'
        cn, an, ct = crypto.encrypt_data_v3(self.key, plaintext)
        recovered = crypto.decrypt_data_v3(self.key, cn, an, ct)
        self.assertEqual(recovered, plaintext)

    def test_roundtrip_with_aad(self):
        plaintext = b"aad-bound metadata"
        aad = b"vault-aad-v3"
        cn, an, ct = crypto.encrypt_data_v3(self.key, plaintext, aad=aad)
        self.assertEqual(crypto.decrypt_data_v3(self.key, cn, an, ct, aad=aad), plaintext)

    def test_wrong_aad_fails(self):
        plaintext = b"aad-bound metadata"
        cn, an, ct = crypto.encrypt_data_v3(self.key, plaintext, aad=b"good")
        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_data_v3(self.key, cn, an, ct, aad=b"bad")

    def test_tamper_ciphertext_fails(self):
        cn, an, ct = crypto.encrypt_data_v3(self.key, b"tamper me")
        bad = bytearray(ct)
        bad[-1] ^= 0x01
        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_data_v3(self.key, cn, an, bytes(bad))

    def test_wrong_key_fails(self):
        cn, an, ct = crypto.encrypt_data_v3(self.key, b"secret")
        other = _make_key("other-key-password!", salt=b"\x55" * 16)
        with self.assertRaises(crypto.CryptoError):
            crypto.decrypt_data_v3(other, cn, an, ct)

    def test_empty_plaintext(self):
        cn, an, ct = crypto.encrypt_data_v3(self.key, b"")
        self.assertEqual(crypto.decrypt_data_v3(self.key, cn, an, ct), b"")

    def test_nonces_are_correct_lengths(self):
        cn, an, ct = crypto.encrypt_data_v3(self.key, b"x")
        self.assertEqual(len(cn), crypto.NONCE_SIZE)
        self.assertEqual(len(an), crypto.NONCE_SIZE)
        self.assertGreater(len(ct), 0)


class StreamV4Tests(unittest.TestCase):
    def setUp(self):
        self.key = _make_key("v4-legacy-stream-pw")

    def _enc(self, plaintext: bytes, chunk_size: int = SMALL_CHUNK) -> bytes:
        out = io.BytesIO()
        with mock.patch.object(crypto, "CHUNK_SIZE", chunk_size):
            crypto.encrypt_stream_v4(self.key, io.BytesIO(plaintext), out)
        return out.getvalue()

    def _dec(self, blob: bytes) -> bytes:
        out = io.BytesIO()
        crypto.decrypt_stream_v4(self.key, io.BytesIO(blob), out)
        return out.getvalue()

    def test_roundtrip_small(self):
        pt = b"v4 stream small payload"
        self.assertEqual(self._dec(self._enc(pt)), pt)

    def test_roundtrip_multi_chunk(self):
        pt = b"0123456789abcdef" * 20  # 320 bytes → multiple SMALL_CHUNK frames
        self.assertEqual(self._dec(self._enc(pt, chunk_size=SMALL_CHUNK)), pt)

    def test_empty_roundtrip(self):
        self.assertEqual(self._dec(self._enc(b"")), b"")

    def test_wrong_key_fails(self):
        blob = self._enc(b"v4 secret")
        wrong = _make_key("wrong-v4-key!!!!!", salt=b"\x66" * 16)
        with self.assertRaises(crypto.CryptoError):
            out = io.BytesIO()
            crypto.decrypt_stream_v4(wrong, io.BytesIO(blob), out)

    def test_bitflip_fails(self):
        blob = self._enc(b"v4 bitflip target payload long enough" * 4)
        # After nonces (16+12=28)
        tampered = bytearray(blob)
        tampered[40] ^= 0x01
        with self.assertRaises(crypto.CryptoError):
            out = io.BytesIO()
            crypto.decrypt_stream_v4(self.key, io.BytesIO(bytes(tampered)), out)


class LegacyV5DecryptPathTests(unittest.TestCase):
    def setUp(self):
        self.key = _make_key("legacy-v5-path-pw!")

    def test_legacy_uncompressed_roundtrip_via_decrypt_stream_v5(self):
        plaintext = b"legacy pre-header v5 payload without magic"
        blob = _build_legacy_v5_stream(self.key, plaintext, compress=False)
        # Must not start with modern magic so dispatcher takes legacy path
        self.assertNotEqual(blob[: len(crypto.STREAM_V5_MAGIC)], crypto.STREAM_V5_MAGIC)
        recovered = _decrypt_v5(self.key, blob)
        self.assertEqual(recovered, plaintext)

    def test_legacy_compressed_roundtrip(self):
        plaintext = b"legacy compressed " * 30
        blob = _build_legacy_v5_stream(self.key, plaintext, compress=True)
        self.assertEqual(_decrypt_v5(self.key, blob), plaintext)

    def test_legacy_wrong_key_fails(self):
        blob = _build_legacy_v5_stream(self.key, b"secret legacy", compress=False)
        wrong = _make_key("not-the-legacy-key", salt=b"\x77" * 16)
        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(wrong, blob)

    def test_legacy_bitflip_fails(self):
        blob = _build_legacy_v5_stream(self.key, b"legacy bitflip body" * 8, compress=False)
        tampered = bytearray(blob)
        # Past flag+nonces (~1+16+12)
        tampered[min(40, len(tampered) - 1)] ^= 0x55
        with self.assertRaises(crypto.CryptoError):
            _decrypt_v5(self.key, bytes(tampered))

    def test_modern_magic_path_still_preferred(self):
        # Smoke: modern encrypt uses magic and is distinct from legacy layout
        modern = _encrypt_v5(self.key, b"modern", compress=False)
        self.assertTrue(modern.startswith(crypto.STREAM_V5_MAGIC))
        legacy = _build_legacy_v5_stream(self.key, b"legacy", compress=False)
        self.assertFalse(legacy.startswith(crypto.STREAM_V5_MAGIC))
        self.assertEqual(_decrypt_v5(self.key, modern), b"modern")
        self.assertEqual(_decrypt_v5(self.key, legacy), b"legacy")


class HelperSanityTests(unittest.TestCase):
    def test_sha256_bytes_known_vector(self):
        # Empty string SHA-256
        self.assertEqual(
            crypto.sha256_bytes(b""),
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        )
        self.assertEqual(
            crypto.sha256_bytes(b"abc"),
            hashlib.sha256(b"abc").hexdigest(),
        )

    def test_sha256_bytes_returns_hex_str(self):
        digest = crypto.sha256_bytes(b"pulse")
        self.assertIsInstance(digest, str)
        self.assertEqual(len(digest), 64)
        int(digest, 16)  # valid hex

    def test_scrypt_memory_bytes_formula(self):
        # 128 * n * r * max(p, 1)
        self.assertEqual(crypto.scrypt_memory_bytes(16, 8, 1), 128 * 16 * 8 * 1)
        self.assertEqual(crypto.scrypt_memory_bytes(2**15, 8, 1), 128 * (2**15) * 8 * 1)
        self.assertEqual(crypto.scrypt_memory_bytes(1024, 8, 0), 128 * 1024 * 8 * 1)  # max(p,1)

    def test_scrypt_memory_scales_with_n(self):
        m_fast = crypto.scrypt_memory_bytes(16, 8, 1)
        m_std = crypto.scrypt_memory_bytes(2**15, 8, 1)
        self.assertGreater(m_std, m_fast)

    def test_split_v3_key_wrong_length(self):
        with self.assertRaises(crypto.CryptoError):
            crypto.split_v3_key(b"too-short")

    def test_chunk_nonce_stable_and_differs_by_index(self):
        base = b"\x01" * 12
        n0 = crypto._chunk_nonce(base, 0)
        n1 = crypto._chunk_nonce(base, 1)
        self.assertEqual(len(n0), crypto.NONCE_SIZE)
        self.assertNotEqual(n0, n1)
        self.assertEqual(crypto._chunk_nonce(base, 0), n0)

    def test_stream_aad_includes_index(self):
        flag = b"\x00"
        cn = b"\x02" * 16
        an = b"\x03" * 12
        a0 = crypto._stream_aad(flag, cn, an, 0)
        a1 = crypto._stream_aad(flag, cn, an, 1)
        self.assertTrue(a0.startswith(crypto.STREAM_V5_MAGIC))
        self.assertNotEqual(a0, a1)
        self.assertEqual(a0[-4:], struct.pack(">I", 0))
        self.assertEqual(a1[-4:], struct.pack(">I", 1))


if HAS_HYPOTHESIS:

    class CryptoDeepHypothesisTests(unittest.TestCase):
        def setUp(self):
            self.key = _make_key("hyp-deep-pw!")

        @given(st.binary(min_size=0, max_size=512), st.booleans())
        @settings(max_examples=20, deadline=None)
        def test_hypothesis_v5_roundtrip(self, plaintext, compress):
            blob = _encrypt_v5(self.key, plaintext, compress=compress, chunk_size=SMALL_CHUNK)
            self.assertEqual(_decrypt_v5(self.key, blob), plaintext)

        @given(st.binary(min_size=0, max_size=256))
        @settings(max_examples=12, deadline=None)
        def test_hypothesis_v3_roundtrip(self, plaintext):
            cn, an, ct = crypto.encrypt_data_v3(self.key, plaintext)
            self.assertEqual(crypto.decrypt_data_v3(self.key, cn, an, ct), plaintext)

        @given(st.binary(min_size=0, max_size=256))
        @settings(max_examples=12, deadline=None)
        def test_hypothesis_v4_roundtrip(self, plaintext):
            out = io.BytesIO()
            with mock.patch.object(crypto, "CHUNK_SIZE", SMALL_CHUNK):
                crypto.encrypt_stream_v4(self.key, io.BytesIO(plaintext), out)
            dec = io.BytesIO()
            crypto.decrypt_stream_v4(self.key, io.BytesIO(out.getvalue()), dec)
            self.assertEqual(dec.getvalue(), plaintext)


if __name__ == "__main__":
    unittest.main()
