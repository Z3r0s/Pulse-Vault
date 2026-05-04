import os
import hashlib
from typing import Tuple
from cryptography.hazmat.primitives.ciphers.aead import AESGCM, ChaCha20Poly1305
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.primitives.kdf.scrypt import Scrypt
from cryptography.hazmat.primitives import hashes

# Constants
VAULT1_MAGIC = b"Z3R0VAULT1"
VAULT2_MAGIC = b"Z3R0VAULT2"
VAULT3_MAGIC = b"PULSEVAULT3"

SALT_SIZE = 16
NONCE_SIZE = 12
KEY_SIZE = 32
KDF_ITERATIONS = 600_000

# Scrypt Params for V3
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
    """V3 Memory-Hard Scrypt KDF generating 64 bytes for cascade"""
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
    V3 Cascade Encryption:
    1. ChaCha20-Poly1305
    2. AES-256-GCM
    Returns: chacha_nonce, aes_nonce, ciphertext
    """
    chacha_key, aes_key = split_v3_key(key64)
    
    # Layer 1: ChaCha20
    chacha_nonce = os.urandom(NONCE_SIZE)
    chacha = ChaCha20Poly1305(chacha_key)
    inner_ciphertext = chacha.encrypt(chacha_nonce, plaintext, aad)
    
    # Layer 2: AES-256-GCM
    aes_nonce = os.urandom(NONCE_SIZE)
    aesgcm = AESGCM(aes_key)
    outer_ciphertext = aesgcm.encrypt(aes_nonce, inner_ciphertext, aad)
    
    return chacha_nonce, aes_nonce, outer_ciphertext

def decrypt_data_v3(key64: bytes, chacha_nonce: bytes, aes_nonce: bytes, ciphertext: bytes, aad: bytes = None) -> bytes:
    """
    V3 Cascade Decryption:
    1. AES-256-GCM
    2. ChaCha20-Poly1305
    """
    chacha_key, aes_key = split_v3_key(key64)
    
    # Layer 1: AES-256-GCM Unwrapping
    aesgcm = AESGCM(aes_key)
    try:
        inner_ciphertext = aesgcm.decrypt(aes_nonce, ciphertext, aad)
    except Exception:
        raise CryptoError("AES Decryption failed. Invalid key or corrupted data.")
        
    # Layer 2: ChaCha20 Unwrapping
    chacha = ChaCha20Poly1305(chacha_key)
    try:
        plaintext = chacha.decrypt(chacha_nonce, inner_ciphertext, aad)
        return plaintext
    except Exception:
        raise CryptoError("ChaCha Decryption failed. Data manipulated after AES layer?")
