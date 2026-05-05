import os
import hashlib
from typing import Tuple
from cryptography.hazmat.primitives.ciphers.aead import AESGCM, ChaCha20Poly1305
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.primitives.kdf.scrypt import Scrypt
from cryptography.hazmat.primitives import hashes

from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes

# Constants
VAULT1_MAGIC = b"Z3R0VAULT1"
VAULT2_MAGIC = b"Z3R0VAULT2"
VAULT3_MAGIC = b"PULSEVAULT3"
VAULT4_MAGIC = b"PULSEVAULT4"

SALT_SIZE = 16
NONCE_SIZE = 12
KEY_SIZE = 32
KDF_ITERATIONS = 600_000

# Scrypt Params for V3/V4
SCRYPT_N = 2**15 # 32768, balanced for speed/memory on desktop
SCRYPT_R = 8
SCRYPT_P = 1
V3_KEY_SIZE = 64 # 32 bytes for ChaCha, 32 bytes for AES

class CryptoError(Exception):
    pass

def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()

def derive_key(password: str, salt: bytes) -> bytes:
    """Legacy PBKDF2 for V1/V2"""
    if not password:
        raise CryptoError("Password cannot be empty.")

    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=KEY_SIZE,
        salt=salt,
        iterations=KDF_ITERATIONS,
    )
    return kdf.derive(password.encode("utf-8"))

def derive_key_v3(password: str, salt: bytes) -> bytes:
    """V3/V4 Memory-Hard Scrypt KDF generating 64 bytes for cascade"""
    if not password:
        raise CryptoError("Password cannot be empty.")
        
    kdf = Scrypt(
        salt=salt,
        length=V3_KEY_SIZE,
        n=SCRYPT_N,
        r=SCRYPT_R,
        p=SCRYPT_P,
    )
    return kdf.derive(password.encode("utf-8"))

def split_v3_key(key: bytes) -> Tuple[bytes, bytes]:
    """Splits the 64 byte key into ChaCha20 key and AES256 key"""
    if len(key) != 64:
        raise CryptoError("V3 Key must be 64 bytes.")
    return key[:32], key[32:]

def encrypt_data(key: bytes, plaintext: bytes, aad: bytes = None) -> Tuple[bytes, bytes]:
    """Legacy V1/V2 Encryption (AES-GCM)"""
    nonce = os.urandom(NONCE_SIZE)
    aesgcm = AESGCM(key)
    ciphertext = aesgcm.encrypt(nonce, plaintext, aad)
    return nonce, ciphertext

def decrypt_data(key: bytes, nonce: bytes, ciphertext: bytes, aad: bytes = None) -> bytes:
    """Legacy V1/V2 Decryption (AES-GCM)"""
    aesgcm = AESGCM(key)
    try:
        return aesgcm.decrypt(nonce, ciphertext, aad)
    except Exception:
        raise CryptoError("Decryption failed. Invalid key, corrupted data, or bad AAD.")

def encrypt_data_v3(key64: bytes, plaintext: bytes, aad: bytes = None) -> Tuple[bytes, bytes, bytes]:
    """
    V3 Cascade Encryption (In-Memory)
    """
    chacha_key, aes_key = split_v3_key(key64)
    chacha_nonce = os.urandom(NONCE_SIZE)
    chacha = ChaCha20Poly1305(chacha_key)
    inner_ciphertext = chacha.encrypt(chacha_nonce, plaintext, aad)
    
    aes_nonce = os.urandom(NONCE_SIZE)
    aesgcm = AESGCM(aes_key)
    outer_ciphertext = aesgcm.encrypt(aes_nonce, inner_ciphertext, aad)
    
    return chacha_nonce, aes_nonce, outer_ciphertext

def decrypt_data_v3(key64: bytes, chacha_nonce: bytes, aes_nonce: bytes, ciphertext: bytes, aad: bytes = None) -> bytes:
    """
    V3 Cascade Decryption (In-Memory)
    """
    chacha_key, aes_key = split_v3_key(key64)
    aesgcm = AESGCM(aes_key)
    try:
        inner_ciphertext = aesgcm.decrypt(aes_nonce, ciphertext, aad)
    except Exception:
        raise CryptoError("AES Decryption failed. Invalid key or corrupted data.")
        
    chacha = ChaCha20Poly1305(chacha_key)
    try:
        plaintext = chacha.decrypt(chacha_nonce, inner_ciphertext, aad)
        return plaintext
    except Exception:
        raise CryptoError("ChaCha Decryption failed. Data manipulated after AES layer?")

CHUNK_SIZE = 1024 * 1024 # 1MB chunks

import lzma

def encrypt_stream_v5(key64: bytes, source_file, target_file, compress: bool = True):
    """
    V5 Streaming Cascade Encryption with LZMA Compression.
    Format: [compress_flag(1)] [chacha_nonce(16)] [aes_nonce(12)] [chunk_len(4) chunk_cipher ...]
    """
    chacha_key, aes_key = split_v3_key(key64)
    
    chacha_nonce = os.urandom(16)
    aes_nonce = os.urandom(NONCE_SIZE)
    
    target_file.write(b'\x01' if compress else b'\x00')
    target_file.write(chacha_nonce)
    target_file.write(aes_nonce)

    # Pre-initialize cipher objects ONCE outside the loop
    # Each construction validates the key — doing it per-chunk wastes CPU
    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)

    # Use preset=3: ~3x faster than 6 with only ~5% less compression ratio
    compressor = lzma.LZMACompressor(format=lzma.FORMAT_XZ, preset=3) if compress else None

    def encrypt_and_write(data: bytes, idx: int):
        # XOR chunk index into nonce bytes for per-chunk uniqueness
        idx_bytes = idx.to_bytes(4, byteorder='big')
        c_nonce_chunk = bytearray(chacha_nonce[:12])
        a_nonce_chunk = bytearray(aes_nonce[:12])
        for i in range(4):
            c_nonce_chunk[i] ^= idx_bytes[i]
            a_nonce_chunk[i] ^= idx_bytes[i]

        inner_ct = chacha.encrypt(bytes(c_nonce_chunk), data, None)
        outer_ct = aesgcm.encrypt(bytes(a_nonce_chunk), inner_ct, None)
        
        target_file.write(len(outer_ct).to_bytes(4, byteorder='big'))
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
    """
    V5 Streaming Cascade Decryption with LZMA Decompression.
    """
    chacha_key, aes_key = split_v3_key(key64)
    
    flag = source_file.read(1)
    if not flag:
        return  # Empty file
        
    is_compressed = (flag == b'\x01')
    
    chacha_nonce = source_file.read(16)
    if len(chacha_nonce) < 16:
        raise CryptoError("V5 Decryption failed. File truncated.")
    aes_nonce = source_file.read(NONCE_SIZE)
    if len(aes_nonce) < NONCE_SIZE:
        raise CryptoError("V5 Decryption failed. Nonce truncated.")

    # Pre-initialize once outside the loop
    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)
    decompressor = lzma.LZMADecompressor(format=lzma.FORMAT_XZ) if is_compressed else None
    
    chunk_index = 0
    while True:
        len_bytes = source_file.read(4)
        if not len_bytes or len(len_bytes) < 4:
            break
        chunk_len = int.from_bytes(len_bytes, byteorder='big')
        
        chunk = source_file.read(chunk_len)
        if len(chunk) < chunk_len:
            raise CryptoError("V5 Cascade Decryption failed. File truncated.")

        idx_bytes = chunk_index.to_bytes(4, byteorder='big')
        c_nonce_chunk = bytearray(chacha_nonce[:12])
        a_nonce_chunk = bytearray(aes_nonce[:12])
        for i in range(4):
            c_nonce_chunk[i] ^= idx_bytes[i]
            a_nonce_chunk[i] ^= idx_bytes[i]
        
        try:
            inner_pt = aesgcm.decrypt(bytes(a_nonce_chunk), chunk, None)
            final_pt = chacha.decrypt(bytes(c_nonce_chunk), inner_pt, None)
        except Exception:
            raise CryptoError("V5 Cascade Decryption failed. Invalid key or corrupted MAC.")
            
        if decompressor:
            try:
                out_pt = decompressor.decompress(final_pt)
                target_file.write(out_pt)
            except Exception:
                raise CryptoError("V5 Decompression failed. Data corrupted.")
        else:
            target_file.write(final_pt)
            
        chunk_index += 1

def encrypt_stream_v4(key64: bytes, source_file, target_file):
    """
    V4 Streaming Cascade Encryption
    Splits file into 1MB chunks. Each chunk is independently encrypted with ChaCha20 + AES-GCM.
    Target file format: [chacha_nonce (16)] [aes_nonce (12)] [chunk0_cipher] [chunk0_tag (16)] [chunk1...]
    """
    chacha_key, aes_key = split_v3_key(key64)
    
    chacha_nonce = os.urandom(16)
    aes_nonce = os.urandom(NONCE_SIZE)
    
    target_file.write(chacha_nonce)
    target_file.write(aes_nonce)

    # Pre-initialize cipher objects once outside the loop
    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)

    chunk_index = 0
    while True:
        chunk = source_file.read(CHUNK_SIZE)
        if not chunk:
            break
            
        idx_bytes = chunk_index.to_bytes(4, byteorder='big')
        c_nonce_chunk = bytearray(chacha_nonce[:12])
        a_nonce_chunk = bytearray(aes_nonce[:12])
        for i in range(4):
            c_nonce_chunk[i] ^= idx_bytes[i]
            a_nonce_chunk[i] ^= idx_bytes[i]
        
        inner_ct = chacha.encrypt(bytes(c_nonce_chunk), chunk, None)
        outer_ct = aesgcm.encrypt(bytes(a_nonce_chunk), inner_ct, None)
        
        target_file.write(len(outer_ct).to_bytes(4, byteorder='big'))
        target_file.write(outer_ct)
        chunk_index += 1


def decrypt_stream_v4(key64: bytes, source_file, target_file):
    """
    V4 Streaming Cascade Decryption
    """
    chacha_key, aes_key = split_v3_key(key64)
    
    chacha_nonce = source_file.read(16)
    if not chacha_nonce or len(chacha_nonce) < 16:
        return # Empty file
    aes_nonce = source_file.read(NONCE_SIZE)
    
    chacha = ChaCha20Poly1305(chacha_key)
    aesgcm = AESGCM(aes_key)
    
    chunk_index = 0
    while True:
        len_bytes = source_file.read(4)
        if not len_bytes or len(len_bytes) < 4:
            break
        chunk_len = int.from_bytes(len_bytes, byteorder='big')
        
        chunk = source_file.read(chunk_len)
        if len(chunk) < chunk_len:
            raise CryptoError("V4 Cascade Decryption failed. File truncated.")
            
        c_nonce_chunk = bytearray(chacha_nonce)
        a_nonce_chunk = bytearray(aes_nonce)
        idx_bytes = chunk_index.to_bytes(4, byteorder='big')
        
        for i in range(4):
            c_nonce_chunk[i] ^= idx_bytes[i]
            a_nonce_chunk[i] ^= idx_bytes[i]
            
        c_nonce_12 = bytes(c_nonce_chunk[:12])
        a_nonce_12 = bytes(a_nonce_chunk[:12])
        
        try:
            inner_pt = aesgcm.decrypt(a_nonce_12, chunk, None)
            final_pt = chacha.decrypt(c_nonce_12, inner_pt, None)
        except Exception:
            raise CryptoError("V4 Cascade Decryption failed. Invalid key or corrupted MAC.")
            
        target_file.write(final_pt)
        chunk_index += 1
