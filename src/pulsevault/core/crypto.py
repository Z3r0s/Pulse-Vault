import hashlib
import lzma
import os
import struct
from typing import Dict, Optional, Tuple

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM, ChaCha20Poly1305
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.primitives.kdf.scrypt import Scrypt


VAULT1_MAGIC = b"Z3R0VAULT1"
STREAM_V5_MAGIC = b"PV5STRM1"

SALT_SIZE = 16
NONCE_SIZE = 12
KEY_SIZE = 32
KDF_ITERATIONS = 600_000

KDF_ALGORITHM_SCRYPT = "scrypt"
USER_SCRYPT_PROFILES = ("standard", "hardened")

SCRYPT_PROFILES: Dict[str, Dict[str, int]] = {
    "fast": {"n": 16, "r": 8, "p": 1},
    "standard": {"n": 2**15, "r": 8, "p": 1},
    "hardened": {"n": 2**20, "r": 8, "p": 1},
}


def active_scrypt_profile() -> str:
    profile = os.environ.get("PULSEVAULT_SCRYPT_PROFILE", "").strip().lower()
    if profile in SCRYPT_PROFILES:
        return profile
    if os.environ.get("PULSEVAULT_TEST_FAST_KDF") == "1":
        return "fast"
    return "standard"


def scrypt_params_for_profile(profile: str) -> Tuple[int, int, int]:
    if profile not in SCRYPT_PROFILES:
        raise ValueError(f"Unknown Scrypt profile: {profile}")

    params = SCRYPT_PROFILES[profile]
    n = params["n"]
    if profile == "fast" and os.environ.get("PULSEVAULT_SCRYPT_N"):
        n = int(os.environ["PULSEVAULT_SCRYPT_N"])
    return n, params["r"], params["p"]


_active_profile = active_scrypt_profile()
SCRYPT_N, SCRYPT_R, SCRYPT_P = scrypt_params_for_profile(_active_profile)
V3_KEY_SIZE = 64

CHUNK_SIZE = 1024 * 1024
MAX_ENCRYPTED_CHUNK_SIZE = 64 * 1024 * 1024


class CryptoError(Exception):
    pass


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def derive_key(password: str, salt: bytes) -> bytes:
    """Legacy PBKDF2 for V1/V2."""
    if not password:
        raise CryptoError("Password cannot be empty.")

    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=KEY_SIZE,
        salt=salt,
        iterations=KDF_ITERATIONS,
    )
    return kdf.derive(password.encode("utf-8"))


def scrypt_memory_bytes(n: int, r: int, p: int) -> int:
    """Approximate peak Scrypt memory use in bytes."""
    return 128 * n * r * max(p, 1)


def kdf_record_from_profile(profile: str) -> Dict[str, object]:
    n, r, p = scrypt_params_for_profile(profile)
    return {
        "algorithm": KDF_ALGORITHM_SCRYPT,
        "profile": profile,
        "n": n,
        "r": r,
        "p": p,
    }


def parse_kdf_record(raw: Dict[str, object]) -> Tuple[str, int, int, int]:
    if raw.get("algorithm") != KDF_ALGORITHM_SCRYPT:
        raise CryptoError("Unsupported vault KDF algorithm.")

    try:
        n = int(raw["n"])
        r = int(raw["r"])
        p = int(raw["p"])
    except (KeyError, TypeError, ValueError) as exc:
        raise CryptoError("Invalid vault KDF parameters.") from exc

    if n <= 0 or r <= 0 or p <= 0:
        raise CryptoError("Invalid vault KDF parameters.")

    profile = str(raw.get("profile") or "standard").strip().lower()
    if profile not in SCRYPT_PROFILES:
        raise CryptoError("Unknown vault Scrypt profile.")

    return profile, n, r, p


def derive_key_scrypt(password: str, salt: bytes, n: int, r: int, p: int) -> bytes:
    if not password:
        raise CryptoError("Password cannot be empty.")

    kdf = Scrypt(
        salt=salt,
        length=V3_KEY_SIZE,
        n=n,
        r=r,
        p=p,
    )
    return kdf.derive(password.encode("utf-8"))


def derive_key_v3(
    password: str,
    salt: bytes,
    n: Optional[int] = None,
    r: Optional[int] = None,
    p: Optional[int] = None,
) -> bytes:
    """Scrypt KDF for V3+ vaults, producing keys for both AEAD layers."""
    return derive_key_scrypt(
        password,
        salt,
        SCRYPT_N if n is None else n,
        SCRYPT_R if r is None else r,
        SCRYPT_P if p is None else p,
    )


def split_v3_key(key: bytes) -> Tuple[bytes, bytes]:
    if len(key) != V3_KEY_SIZE:
        raise CryptoError("V3 key must be 64 bytes.")
    return key[:32], key[32:]


def encrypt_data(key: bytes, plaintext: bytes, aad: bytes = None) -> Tuple[bytes, bytes]:
    """Legacy V1/V2 AES-GCM encryption."""
    nonce = os.urandom(NONCE_SIZE)
    ciphertext = AESGCM(key).encrypt(nonce, plaintext, aad)
    return nonce, ciphertext


def decrypt_data(key: bytes, nonce: bytes, ciphertext: bytes, aad: bytes = None) -> bytes:
    """Legacy V1/V2 AES-GCM decryption."""
    try:
        return AESGCM(key).decrypt(nonce, ciphertext, aad)
    except Exception as exc:
        raise CryptoError("Decryption failed. Invalid key, corrupted data, or bad AAD.") from exc


def encrypt_data_v3(key64: bytes, plaintext: bytes, aad: bytes = None) -> Tuple[bytes, bytes, bytes]:
    """In-memory cascade encryption used by metadata and legacy V3 data blocks."""
    chacha_key, aes_key = split_v3_key(key64)
    chacha_nonce = os.urandom(NONCE_SIZE)
    aes_nonce = os.urandom(NONCE_SIZE)

    inner_ciphertext = ChaCha20Poly1305(chacha_key).encrypt(chacha_nonce, plaintext, aad)
    outer_ciphertext = AESGCM(aes_key).encrypt(aes_nonce, inner_ciphertext, aad)
    return chacha_nonce, aes_nonce, outer_ciphertext


def decrypt_data_v3(
    key64: bytes,
    chacha_nonce: bytes,
    aes_nonce: bytes,
    ciphertext: bytes,
    aad: bytes = None,
) -> bytes:
    """In-memory cascade decryption used by metadata and legacy V3 data blocks."""
    chacha_key, aes_key = split_v3_key(key64)
    try:
        inner_ciphertext = AESGCM(aes_key).decrypt(aes_nonce, ciphertext, aad)
        return ChaCha20Poly1305(chacha_key).decrypt(chacha_nonce, inner_ciphertext, aad)
    except Exception as exc:
        raise CryptoError("Cascade decryption failed. Invalid key or corrupted data.") from exc


def _chunk_nonce(base_nonce: bytes, idx: int) -> bytes:
    if idx >= 2**32:
        raise CryptoError("Too many chunks for one encrypted stream.")

    idx_bytes = idx.to_bytes(4, byteorder="big")
    nonce = bytearray(base_nonce[:NONCE_SIZE])
    for pos in range(4):
        nonce[pos] ^= idx_bytes[pos]
    return bytes(nonce)


def _stream_aad(flag: bytes, chacha_nonce: bytes, aes_nonce: bytes, idx: int) -> bytes:
    return STREAM_V5_MAGIC + flag + chacha_nonce + aes_nonce + struct.pack(">I", idx)


def encrypt_stream_v5(key64: bytes, source_file, target_file, compress: bool = True):
    """
    V5 streaming cascade encryption with LZMA compression.

    Format:
        magic(8) | compress_flag(1) | chacha_nonce(16) | aes_nonce(12)
        repeated: chunk_len(4) | chunk_cipher
    """
    chacha_key, aes_key = split_v3_key(key64)
    chacha_nonce = os.urandom(16)
    aes_nonce = os.urandom(NONCE_SIZE)
    flag = b"\x01" if compress else b"\x00"

    target_file.write(STREAM_V5_MAGIC)
    target_file.write(flag)
    target_file.write(chacha_nonce)
    target_file.write(aes_nonce)

    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)
    compressor = lzma.LZMACompressor(format=lzma.FORMAT_XZ, preset=1) if compress else None

    def encrypt_and_write(data: bytes, idx: int):
        aad = _stream_aad(flag, chacha_nonce, aes_nonce, idx)
        inner_ct = chacha.encrypt(_chunk_nonce(chacha_nonce, idx), data, aad)
        outer_ct = aesgcm.encrypt(_chunk_nonce(aes_nonce, idx), inner_ct, aad)
        target_file.write(len(outer_ct).to_bytes(4, byteorder="big"))
        target_file.write(outer_ct)

    chunk_index = 0
    while True:
        raw_chunk = source_file.read(CHUNK_SIZE)
        if not raw_chunk:
            break

        chunk = compressor.compress(raw_chunk) if compressor else raw_chunk
        if chunk:
            encrypt_and_write(chunk, chunk_index)
            chunk_index += 1

    if compressor:
        chunk = compressor.flush()
        if chunk:
            encrypt_and_write(chunk, chunk_index)


def decrypt_stream_v5(key64: bytes, source_file, target_file):
    """V5 streaming cascade decryption with LZMA decompression."""
    chacha_key, aes_key = split_v3_key(key64)

    magic = source_file.read(len(STREAM_V5_MAGIC))
    if not magic:
        return
    if magic != STREAM_V5_MAGIC:
        _decrypt_legacy_stream_v5(key64, magic, source_file, target_file)
        return

    flag = source_file.read(1)
    if flag not in {b"\x00", b"\x01"}:
        raise CryptoError("V5 decryption failed. Invalid compression flag.")

    chacha_nonce = source_file.read(16)
    if len(chacha_nonce) < 16:
        raise CryptoError("V5 decryption failed. File truncated.")

    aes_nonce = source_file.read(NONCE_SIZE)
    if len(aes_nonce) < NONCE_SIZE:
        raise CryptoError("V5 decryption failed. Nonce truncated.")

    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)
    decompressor = lzma.LZMADecompressor(format=lzma.FORMAT_XZ) if flag == b"\x01" else None

    chunk_index = 0
    while True:
        len_bytes = source_file.read(4)
        if not len_bytes:
            break
        if len(len_bytes) < 4:
            raise CryptoError("V5 decryption failed. Truncated chunk length.")

        chunk_len = int.from_bytes(len_bytes, byteorder="big")
        if chunk_len <= 0 or chunk_len > MAX_ENCRYPTED_CHUNK_SIZE:
            raise CryptoError("V5 decryption failed. Invalid chunk length.")

        chunk = source_file.read(chunk_len)
        if len(chunk) < chunk_len:
            raise CryptoError("V5 cascade decryption failed. File truncated.")

        aad = _stream_aad(flag, chacha_nonce, aes_nonce, chunk_index)
        try:
            inner_pt = aesgcm.decrypt(_chunk_nonce(aes_nonce, chunk_index), chunk, aad)
            final_pt = chacha.decrypt(_chunk_nonce(chacha_nonce, chunk_index), inner_pt, aad)
        except Exception as exc:
            raise CryptoError("V5 cascade decryption failed. Invalid key or corrupted MAC.") from exc

        if decompressor:
            try:
                target_file.write(decompressor.decompress(final_pt))
            except Exception as exc:
                raise CryptoError("V5 decompression failed. Data corrupted.") from exc
        else:
            target_file.write(final_pt)

        chunk_index += 1

    if decompressor and not decompressor.eof:
        raise CryptoError("V5 decompression failed. Compressed stream was truncated.")


def _decrypt_legacy_stream_v5(key64: bytes, first_bytes: bytes, source_file, target_file):
    """Read the pre-header V5 stream format produced by older PulseVault builds."""
    chacha_key, aes_key = split_v3_key(key64)
    flag = first_bytes[:1]
    if flag not in {b"\x00", b"\x01"}:
        raise CryptoError("V5 decryption failed. Unknown stream header.")

    chacha_nonce = first_bytes[1:] + source_file.read(9)
    if len(chacha_nonce) < 16:
        raise CryptoError("V5 decryption failed. File truncated.")

    aes_nonce = source_file.read(NONCE_SIZE)
    if len(aes_nonce) < NONCE_SIZE:
        raise CryptoError("V5 decryption failed. Nonce truncated.")

    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)
    decompressor = lzma.LZMADecompressor(format=lzma.FORMAT_XZ) if flag == b"\x01" else None

    chunk_index = 0
    while True:
        len_bytes = source_file.read(4)
        if not len_bytes:
            break
        if len(len_bytes) < 4:
            raise CryptoError("V5 decryption failed. Truncated chunk length.")

        chunk_len = int.from_bytes(len_bytes, byteorder="big")
        if chunk_len <= 0 or chunk_len > MAX_ENCRYPTED_CHUNK_SIZE:
            raise CryptoError("V5 decryption failed. Invalid chunk length.")

        chunk = source_file.read(chunk_len)
        if len(chunk) < chunk_len:
            raise CryptoError("V5 cascade decryption failed. File truncated.")

        try:
            inner_pt = aesgcm.decrypt(_chunk_nonce(aes_nonce, chunk_index), chunk, None)
            final_pt = chacha.decrypt(_chunk_nonce(chacha_nonce, chunk_index), inner_pt, None)
        except Exception as exc:
            raise CryptoError("V5 cascade decryption failed. Invalid key or corrupted MAC.") from exc

        if decompressor:
            try:
                target_file.write(decompressor.decompress(final_pt))
            except Exception as exc:
                raise CryptoError("V5 decompression failed. Data corrupted.") from exc
        else:
            target_file.write(final_pt)

        chunk_index += 1

    if decompressor and not decompressor.eof:
        raise CryptoError("V5 decompression failed. Compressed stream was truncated.")


def encrypt_stream_v4(key64: bytes, source_file, target_file):
    """Legacy V4 streaming cascade encryption."""
    chacha_key, aes_key = split_v3_key(key64)
    chacha_nonce = os.urandom(16)
    aes_nonce = os.urandom(NONCE_SIZE)

    target_file.write(chacha_nonce)
    target_file.write(aes_nonce)

    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)

    chunk_index = 0
    while True:
        chunk = source_file.read(CHUNK_SIZE)
        if not chunk:
            break

        inner_ct = chacha.encrypt(_chunk_nonce(chacha_nonce, chunk_index), chunk, None)
        outer_ct = aesgcm.encrypt(_chunk_nonce(aes_nonce, chunk_index), inner_ct, None)
        target_file.write(len(outer_ct).to_bytes(4, byteorder="big"))
        target_file.write(outer_ct)
        chunk_index += 1


def decrypt_stream_v4(key64: bytes, source_file, target_file):
    """Legacy V4 streaming cascade decryption."""
    chacha_key, aes_key = split_v3_key(key64)
    chacha_nonce = source_file.read(16)
    if not chacha_nonce:
        return
    if len(chacha_nonce) < 16:
        raise CryptoError("V4 decryption failed. File truncated.")

    aes_nonce = source_file.read(NONCE_SIZE)
    if len(aes_nonce) < NONCE_SIZE:
        raise CryptoError("V4 decryption failed. Nonce truncated.")

    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)

    chunk_index = 0
    while True:
        len_bytes = source_file.read(4)
        if not len_bytes:
            break
        if len(len_bytes) < 4:
            raise CryptoError("V4 decryption failed. Truncated chunk length.")

        chunk_len = int.from_bytes(len_bytes, byteorder="big")
        if chunk_len <= 0 or chunk_len > MAX_ENCRYPTED_CHUNK_SIZE:
            raise CryptoError("V4 decryption failed. Invalid chunk length.")

        chunk = source_file.read(chunk_len)
        if len(chunk) < chunk_len:
            raise CryptoError("V4 cascade decryption failed. File truncated.")

        try:
            inner_pt = aesgcm.decrypt(_chunk_nonce(aes_nonce, chunk_index), chunk, None)
            final_pt = chacha.decrypt(_chunk_nonce(chacha_nonce, chunk_index), inner_pt, None)
        except Exception as exc:
            raise CryptoError("V4 cascade decryption failed. Invalid key or corrupted MAC.") from exc

        target_file.write(final_pt)
        chunk_index += 1
