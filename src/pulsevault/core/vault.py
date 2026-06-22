import os
import json
import base64
import copy
import hmac
import hashlib
import zipfile
import tempfile
import shutil
import time
import uuid
import queue
import threading
from contextlib import contextmanager
from pathlib import Path, PurePosixPath
from typing import Optional, Dict, Any, List, Callable

from pulsevault.core.crypto import (
    VAULT1_MAGIC,
    SALT_SIZE,
    NONCE_SIZE,
    SCRYPT_PROFILES,
    active_scrypt_profile,
    derive_key,
    derive_key_scrypt,
    derive_key_v3,
    encrypt_data,
    decrypt_data,
    encrypt_data_v3,
    decrypt_data_v3,
    encrypt_stream_v4,
    decrypt_stream_v4,
    encrypt_stream_v5,
    decrypt_stream_v5,
    parse_kdf_record,
    scrypt_params_for_profile,
    CryptoError,
)

class VaultError(Exception):
    pass


class HashWriter:
    def __init__(self):
        self.hasher = hashlib.sha256()
        self.size = 0

    def write(self, data: bytes):
        self.hasher.update(data)
        self.size += len(data)
        return len(data)

    def hexdigest(self) -> str:
        return self.hasher.hexdigest()


class QueueWriter:
    def __init__(self, maxsize: int = 8):
        self.queue = queue.Queue(maxsize=maxsize)
        self.cancelled = threading.Event()
        self._sentinel = object()

    def write(self, data: bytes):
        if data:
            payload = bytes(data)
            while not self.cancelled.is_set():
                try:
                    self.queue.put(payload, timeout=0.1)
                    break
                except queue.Full:
                    continue
            else:
                raise VaultError("Encryption pipeline was cancelled.")
        return len(data)

    def close(self):
        while not self.cancelled.is_set():
            try:
                self.queue.put(self._sentinel, timeout=0.1)
                return
            except queue.Full:
                continue

    def cancel(self):
        self.cancelled.set()


class QueueReader:
    def __init__(self, writer: QueueWriter):
        self.writer = writer
        self.buffer = bytearray()
        self.closed = False

    def read(self, size: int = -1) -> bytes:
        if size is None or size < 0:
            chunks = [bytes(self.buffer)]
            self.buffer.clear()
            while not self.closed:
                item = self.writer.queue.get()
                if item is self.writer._sentinel:
                    self.closed = True
                    break
                chunks.append(item)
            return b"".join(chunks)

        while len(self.buffer) < size and not self.closed:
            item = self.writer.queue.get()
            if item is self.writer._sentinel:
                self.closed = True
                break
            self.buffer.extend(item)

        out = bytes(self.buffer[:size])
        del self.buffer[:size]
        return out


class BytesReader:
    def __init__(self, data: bytes):
        self.data = data
        self.offset = 0

    def read(self, size: int = -1) -> bytes:
        if size is None or size < 0:
            size = len(self.data) - self.offset
        chunk = self.data[self.offset:self.offset + size]
        self.offset += len(chunk)
        return chunk


def encrypt_from_decrypting_source(decrypt_func: Callable[[QueueWriter], None], key: bytes, target):
    writer = QueueWriter()
    reader = QueueReader(writer)
    error = []

    def worker():
        try:
            decrypt_func(writer)
        except Exception as exc:
            error.append(exc)
        finally:
            writer.close()

    thread = threading.Thread(target=worker, daemon=True)
    thread.start()
    try:
        encrypt_stream_v5(key, reader, target)
    except Exception:
        writer.cancel()
        thread.join(timeout=5)
        raise

    thread.join(timeout=5)
    if thread.is_alive():
        writer.cancel()
        raise VaultError("Encryption pipeline did not stop cleanly.")

    if error:
        raise error[0]

FORMAT_V5 = b"PULSEVAULT5_COMPRESSED_CASCADE"
FORMAT_V4 = b"PULSEVAULT4_CASCADE"
FORMAT_V3 = b"PULSEVAULT3_CASCADE"

MAX_ZIP_ENTRIES = 20_000
MAX_FORMAT_SIZE = 128
MAX_KDF_JSON_SIZE = 512
MAX_METADATA_SIZE = 16 * 1024 * 1024
MAX_DATA_BLOB_SIZE = 512 * 1024 * 1024 * 1024
MAX_FOLDER_FILES = 10_000
MAX_FOLDER_BYTES = 25 * 1024 * 1024 * 1024

def now_unix() -> int:
    return int(time.time())

def b64e(data: bytes) -> str:
    return base64.b64encode(data).decode("utf-8")

def b64d(data: str) -> bytes:
    return base64.b64decode(data.encode("utf-8"))

def safe_filename(name: str) -> str:
    name = name.strip().replace("\\", "_").replace("/", "_")
    name = "".join("_" if ord(ch) < 32 or ch in '<>:"|?*' else ch for ch in name)
    name = name.rstrip(" .")
    if not name or name in {".", ".."}:
        raise VaultError("Invalid filename.")
    reserved = {
        "CON", "PRN", "AUX", "NUL",
        "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
        "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
    }
    if Path(name).stem.upper() in reserved:
        raise VaultError("Filename is reserved by the operating system.")
    return name

def secure_unlink(path: Path):
    """Best-effort overwrite before deleting temporary plaintext files."""
    try:
        if path.is_symlink():
            path.unlink(missing_ok=True)
            return
        if path.exists() and path.is_file():
            size = path.stat().st_size
            with open(path, "r+b", buffering=0) as f:
                remaining = size
                zero_block = b"\x00" * (1024 * 1024)
                while remaining > 0:
                    block = zero_block[:min(len(zero_block), remaining)]
                    f.write(block)
                    remaining -= len(block)
                f.flush()
                os.fsync(f.fileno())
    except Exception:
        pass
    try:
        path.unlink(missing_ok=True)
    except Exception:
        pass

def stream_sha256(file_path: Path) -> str:
    h = hashlib.sha256()
    with open(file_path, "rb") as fh:
        while True:
            block = fh.read(1024 * 1024)
            if not block:
                break
            h.update(block)
    return h.hexdigest()


def make_private_temp(target_path: Path, suffix: str):
    # Same-directory temps keep os.replace atomic and avoid predictable .tmp names.
    target_path.parent.mkdir(parents=True, exist_ok=True)
    prefix = f".{target_path.name}.{uuid.uuid4().hex}."
    fd, raw_path = tempfile.mkstemp(prefix=prefix, suffix=suffix, dir=target_path.parent)
    return Path(raw_path), os.fdopen(fd, "w+b")


def ensure_not_symlink(path: Path, label: str):
    if path.is_symlink():
        raise VaultError(f"Refusing to write through a symbolic link: {label}")


def validate_zip_member(info: zipfile.ZipInfo):
    # Vault ZIP entries are never extracted directly, but strict names keep the
    # parser honest and make malformed containers fail early.
    name = info.filename
    parts = PurePosixPath(name).parts
    if name.startswith(("/", "\\")) or "\\" in name or ".." in parts:
        raise VaultError("Invalid vault entry path.")
    if info.compress_type != zipfile.ZIP_STORED:
        raise VaultError("Invalid vault entry compression.")
    if name == "salt.bin" and info.file_size != SALT_SIZE:
        raise VaultError("Invalid vault salt size.")
    if name == "format.txt" and info.file_size > MAX_FORMAT_SIZE:
        raise VaultError("Invalid vault format marker size.")
    if name == "kdf.json" and info.file_size > MAX_KDF_JSON_SIZE:
        raise VaultError("Invalid vault KDF record size.")
    if name == "metadata.enc" and info.file_size > MAX_METADATA_SIZE:
        raise VaultError("Vault metadata is too large.")
    if name.startswith("data/") and info.file_size > MAX_DATA_BLOB_SIZE:
        raise VaultError("Vault data entry is too large.")


def validate_vault_zip(z: zipfile.ZipFile):
    infos = z.infolist()
    if len(infos) > MAX_ZIP_ENTRIES:
        raise VaultError("Vault contains too many internal entries.")

    seen = set()
    for info in infos:
        if info.filename in seen:
            raise VaultError("Vault contains duplicate internal entries.")
        seen.add(info.filename)
        validate_zip_member(info)

    if "salt.bin" not in seen or "metadata.enc" not in seen:
        raise VaultError("Invalid vault structure.")


def read_zip_entry(z: zipfile.ZipFile, name: str, max_size: int, exact_size: int = None) -> bytes:
    try:
        info = z.getinfo(name)
    except KeyError:
        raise VaultError(f"Missing vault entry: {name}")

    if exact_size is not None and info.file_size != exact_size:
        raise VaultError(f"Invalid size for vault entry: {name}")
    if info.file_size > max_size:
        raise VaultError(f"Vault entry is too large: {name}")

    data = z.read(name)
    if exact_size is not None and len(data) != exact_size:
        raise VaultError(f"Invalid size for vault entry: {name}")
    if len(data) > max_size:
        raise VaultError(f"Vault entry is too large: {name}")
    return data


@contextmanager
def checked_zip(path: Path):
    with zipfile.ZipFile(path, "r") as z:
        validate_vault_zip(z)
        yield z

class EncryptedVault:
    def __init__(self, vault_path: Path):
        self.vault_path = vault_path
        self.salt: Optional[bytes] = None
        self.key: Optional[bytes] = None
        self.data: Dict[str, Any] = self.default_data()
        self.version = 5
        self.scrypt_profile = active_scrypt_profile()
        self.kdf_n, self.kdf_r, self.kdf_p = scrypt_params_for_profile(self.scrypt_profile)
    @staticmethod
    def default_data() -> Dict[str, Any]:
        return {
            "version": 5,
            "created_at": now_unix(),
            "updated_at": now_unix(),
            "files": {},
        }

    @property
    def is_unlocked(self) -> bool:
        return self.key is not None and self.salt is not None

    def _set_kdf_params(self, profile: str, n: Optional[int] = None, r: Optional[int] = None, p: Optional[int] = None):
        if profile not in SCRYPT_PROFILES:
            raise VaultError("Unknown Scrypt profile.")
        default_n, default_r, default_p = scrypt_params_for_profile(profile)
        self.scrypt_profile = profile
        self.kdf_n = default_n if n is None else n
        self.kdf_r = default_r if r is None else r
        self.kdf_p = default_p if p is None else p

    def _derive_v3_key(self, password: str, salt: bytes) -> bytes:
        return derive_key_scrypt(password, salt, self.kdf_n, self.kdf_r, self.kdf_p)

    def _kdf_record(self) -> Dict[str, object]:
        return {
            "algorithm": "scrypt",
            "profile": self.scrypt_profile,
            "n": self.kdf_n,
            "r": self.kdf_r,
            "p": self.kdf_p,
        }

    def _load_kdf_record(self, raw: Optional[Dict[str, object]]):
        if raw is None:
            self._set_kdf_params(active_scrypt_profile())
            return
        try:
            profile, n, r, p = parse_kdf_record(raw)
        except CryptoError as exc:
            raise VaultError("Invalid vault KDF record.") from exc
        self._set_kdf_params(profile, n=n, r=r, p=p)

    def create(self, password: str, carrier_path: Optional[Path] = None, scrypt_profile: Optional[str] = None):
        if self.vault_path.exists():
            raise VaultError("A vault already exists at that location.")

        profile = scrypt_profile or active_scrypt_profile()
        if profile not in SCRYPT_PROFILES:
            raise VaultError("Unknown Scrypt profile.")

        self.carrier_path = carrier_path if (carrier_path and carrier_path.exists()) else None
        if self.carrier_path:
            self.carrier_offset = self.carrier_path.stat().st_size
        self._set_kdf_params(profile)

        self.salt = os.urandom(SALT_SIZE)
        self.key = derive_key_scrypt(password, self.salt, self.kdf_n, self.kdf_r, self.kdf_p)
        self.data = self.default_data()
        self.version = 5
        self.save()

    def unlock(self, password: str):
        if not self.vault_path.exists():
            raise VaultError("Vault file does not exist.")

        # A small header probe is enough to catch legacy raw vaults.
        with open(self.vault_path, "rb") as f:
            header_chunk = f.read(1024 * 1024 * 5)
        
        if header_chunk.startswith(VAULT1_MAGIC):
            # V1 predates the ZIP container and stores one encrypted JSON blob.
            raw = self.vault_path.read_bytes()
            self._unlock_v1(password, raw)
            return

        if not zipfile.is_zipfile(self.vault_path):
            raise VaultError("Invalid vault format. ZIP container corrupted.")

        with checked_zip(self.vault_path) as z:
            infolist = z.infolist()
            if infolist:
                self.carrier_offset = min(info.header_offset for info in infolist)
            else:
                self.carrier_offset = 0

            format_txt = b""
            if "format.txt" in z.namelist():
                format_txt = read_zip_entry(z, "format.txt", MAX_FORMAT_SIZE)
            is_v5 = format_txt == FORMAT_V5
            is_v4 = format_txt == FORMAT_V4
            is_v3 = format_txt == FORMAT_V3
            is_v2 = not is_v3 and not is_v4 and not is_v5

            salt = read_zip_entry(z, "salt.bin", SALT_SIZE, exact_size=SALT_SIZE)
            enc_meta = read_zip_entry(z, "metadata.enc", MAX_METADATA_SIZE)
            kdf_raw = None
            if "kdf.json" in z.namelist():
                try:
                    kdf_raw = json.loads(
                        read_zip_entry(z, "kdf.json", MAX_KDF_JSON_SIZE).decode("utf-8")
                    )
                except (UnicodeDecodeError, json.JSONDecodeError) as exc:
                    raise VaultError("Invalid vault KDF record.") from exc

        if is_v5 or is_v4 or is_v3:
            self._load_kdf_record(kdf_raw)

        if is_v5:
            self._unlock_v5(password, salt, enc_meta)
        elif is_v4:
            self._unlock_v4(password, salt, enc_meta)
        elif is_v3:
            self._unlock_v3(password, salt, enc_meta)
        else:
            self._unlock_v2(password, salt, enc_meta)

    def _unlock_v1(self, password: str, raw: bytes):
        offset = len(VAULT1_MAGIC)
        if len(raw) < len(VAULT1_MAGIC) + SALT_SIZE + NONCE_SIZE:
            raise VaultError("Vault file is too small or corrupted.")

        salt = raw[offset:offset + SALT_SIZE]
        offset += SALT_SIZE
        nonce = raw[offset:offset + NONCE_SIZE]
        offset += NONCE_SIZE
        ciphertext = raw[offset:]

        key = derive_key(password, salt)
        try:
            plaintext = decrypt_data(key, nonce, ciphertext, VAULT1_MAGIC)
        except CryptoError:
            raise VaultError("Invalid password or corrupted vault.")

        try:
            loaded = json.loads(plaintext.decode("utf-8"))
        except Exception:
            raise VaultError("Vault decrypted, but internal data is invalid.")

        self._load_data(loaded, password, salt, key, version=1)

    def _unlock_v2(self, password: str, salt: bytes, enc_meta: bytes):
        key = derive_key(password, salt)
        if len(enc_meta) < NONCE_SIZE:
            raise VaultError("Corrupted metadata.")
        
        nonce = enc_meta[:NONCE_SIZE]
        ciphertext = enc_meta[NONCE_SIZE:]

        try:
            plaintext = decrypt_data(key, nonce, ciphertext)
        except CryptoError:
            raise VaultError("Invalid password or corrupted vault.")

        loaded = json.loads(plaintext.decode("utf-8"))
        self._load_data(loaded, password, salt, key, version=2)

    def _unlock_v3(self, password: str, salt: bytes, enc_meta: bytes):
        key = self._derive_v3_key(password, salt)
        if len(enc_meta) < (NONCE_SIZE * 2):
            raise VaultError("Corrupted V3 metadata.")

        chacha_nonce = enc_meta[:NONCE_SIZE]
        aes_nonce = enc_meta[NONCE_SIZE:(NONCE_SIZE*2)]
        ciphertext = enc_meta[(NONCE_SIZE*2):]

        try:
            plaintext = decrypt_data_v3(key, chacha_nonce, aes_nonce, ciphertext)
        except CryptoError:
            raise VaultError("Invalid password or corrupted vault (Cascade Layer Failed).")

        loaded = json.loads(plaintext.decode("utf-8"))
        self._load_data(loaded, password, salt, key, version=3)

    def _unlock_v4(self, password: str, salt: bytes, enc_meta: bytes):
        key = self._derive_v3_key(password, salt)
        # In V4, metadata is still encrypted with V3 in-memory method for speed
        if len(enc_meta) < (NONCE_SIZE * 2):
            raise VaultError("Corrupted V4 metadata.")
            
        chacha_nonce = enc_meta[:NONCE_SIZE]
        aes_nonce = enc_meta[NONCE_SIZE:(NONCE_SIZE*2)]
        ciphertext = enc_meta[(NONCE_SIZE*2):]
        
        try:
            plaintext = decrypt_data_v3(key, chacha_nonce, aes_nonce, ciphertext)
        except CryptoError:
            raise VaultError("Invalid password or corrupted vault (Cascade Layer Failed).")
            
        loaded = json.loads(plaintext.decode("utf-8"))
        self._load_data(loaded, password, salt, key, version=4)

    def _unlock_v5(self, password: str, salt: bytes, enc_meta: bytes):
        key = self._derive_v3_key(password, salt)
        if len(enc_meta) < (NONCE_SIZE * 2):
            raise VaultError("Corrupted V5 metadata.")
            
        chacha_nonce = enc_meta[:NONCE_SIZE]
        aes_nonce = enc_meta[NONCE_SIZE:(NONCE_SIZE*2)]
        ciphertext = enc_meta[(NONCE_SIZE*2):]
        
        try:
            plaintext = decrypt_data_v3(key, chacha_nonce, aes_nonce, ciphertext)
        except CryptoError:
            raise VaultError("Invalid password or corrupted vault (Cascade Layer Failed).")
            
        loaded = json.loads(plaintext.decode("utf-8"))
        self._load_data(loaded, password, salt, key, version=5)

    def _load_data(self, loaded: dict, password: str, salt: bytes, key: bytes, version: int):
        if "files" not in loaded or not isinstance(loaded["files"], dict):
            loaded["files"] = {}

        self.salt = salt
        self.key = key
        self.data = loaded
        self.version = version

    def migrate_to_current_format(self, password: str):
        if self.version >= 5:
            return
        if not self._password_matches_current_key(password):
            raise VaultError("Password does not match unlocked vault.")
        self.change_password(password, password)

    def lock(self):
        self.salt = None
        self.key = None
        self.data = self.default_data()

    def _password_matches_current_key(self, password: str) -> bool:
        if not self.salt or not self.key:
            return False
        try:
            candidate = (
                derive_key(password, self.salt)
                if self.version < 3
                else self._derive_v3_key(password, self.salt)
            )
        except CryptoError:
            return False
        return hmac.compare_digest(candidate, self.key)

    def save(self):
        if not self.key or not self.salt:
            raise VaultError("Vault is locked.")

        old_data = copy.deepcopy(self.data)
        old_version = self.version
        self.data["updated_at"] = now_unix()
        self.data["version"] = 5
        self.version = 5

        def write_entries(z):
            if self.vault_path.exists() and zipfile.is_zipfile(self.vault_path):
                with checked_zip(self.vault_path) as old_z:
                    for item in old_z.infolist():
                        if item.filename.startswith("data/"):
                            keep = False
                            for meta in self.data["files"].values():
                                if meta.get("internal_id") and item.filename == f"data/{meta['internal_id']}.enc":
                                    keep = True
                                    break

                            if keep:
                                with old_z.open(item.filename, "r") as source:
                                    with z.open(item, "w") as target:
                                        shutil.copyfileobj(source, target)

            for fname, meta in list(self.data["files"].items()):
                if "content" not in meta:
                    continue

                content_bytes = b64d(meta["content"])
                internal_id = str(uuid.uuid4())
                with z.open(f"data/{internal_id}.enc", "w") as target:
                    encrypt_stream_v5(self.key, BytesReader(content_bytes), target)

                meta["internal_id"] = internal_id
                del meta["content"]

        try:
            self._write_vault_zip(self.salt, self.key, write_entries)
        except Exception:
            self.data = old_data
            self.version = old_version
            raise

    def _write_vault_zip(self, salt: bytes, key: bytes, write_entries: Callable):
        ensure_not_symlink(self.vault_path, self.vault_path.name)
        temp_zip_path = None
        tmp_path = None

        try:
            temp_zip_path, temp_zip = make_private_temp(self.vault_path, ".zip")
            with temp_zip:
                with zipfile.ZipFile(temp_zip, "w", zipfile.ZIP_STORED) as z:
                    z.writestr("salt.bin", salt)
                    z.writestr("format.txt", FORMAT_V5)
                    z.writestr("kdf.json", json.dumps(self._kdf_record(), indent=2).encode("utf-8"))
                    write_entries(z)
                    plaintext = json.dumps(self.data, indent=2).encode("utf-8")
                    c_nonce, a_nonce, ciphertext = encrypt_data_v3(key, plaintext)
                    z.writestr("metadata.enc", c_nonce + a_nonce + ciphertext)

            tmp_path, out = make_private_temp(self.vault_path, ".tmp")
            with out:
                self._write_carrier_prefix(out)
                with open(temp_zip_path, "rb") as z_in:
                    shutil.copyfileobj(z_in, out)

            ensure_not_symlink(self.vault_path, self.vault_path.name)
            tmp_path.replace(self.vault_path)
        finally:
            if temp_zip_path:
                temp_zip_path.unlink(missing_ok=True)
            if tmp_path:
                tmp_path.unlink(missing_ok=True)

    def _write_carrier_prefix(self, target):
        carrier_path = getattr(self, "carrier_path", None)
        if carrier_path and carrier_path.exists():
            with open(carrier_path, "rb") as c_in:
                shutil.copyfileobj(c_in, target)
            return

        if getattr(self, "carrier_offset", 0) > 0 and self.vault_path.exists():
            with open(self.vault_path, "rb") as c_in:
                bytes_left = self.carrier_offset
                while bytes_left > 0:
                    chunk = c_in.read(min(bytes_left, 1024 * 1024 * 4))
                    if not chunk:
                        break
                    target.write(chunk)
                    bytes_left -= len(chunk)

    def list_files(self) -> List[str]:
        return sorted(self.data.get("files", {}).keys(), key=lambda s: s.lower())

    def get_file_meta(self, filename: str) -> Dict[str, Any]:
        files = self.data.get("files", {})
        if filename not in files:
            raise VaultError("File not found in vault.")
        return files[filename]

    def _decrypt_file_to_target(self, item: Dict[str, Any], target):
        if "content" in item:
            target.write(b64d(item["content"]))
            return

        internal_id = item.get("internal_id")
        if not internal_id:
            raise VaultError("File metadata missing content reference.")
        if not zipfile.is_zipfile(self.vault_path):
            raise VaultError("Vault is not a valid zip container.")

        with checked_zip(self.vault_path) as z:
            format_txt = read_zip_entry(z, "format.txt", MAX_FORMAT_SIZE) if "format.txt" in z.namelist() else b""
            source_name = f"data/{internal_id}.enc"
            if source_name not in z.namelist():
                raise VaultError("Internal data file missing from vault.")

            if format_txt == FORMAT_V5:
                with z.open(source_name, "r") as source:
                    try:
                        decrypt_stream_v5(self.key, source, target)
                    except CryptoError as exc:
                        raise VaultError("Failed to decrypt file (stream authentication failed).") from exc
            elif format_txt == FORMAT_V4:
                with z.open(source_name, "r") as source:
                    try:
                        decrypt_stream_v4(self.key, source, target)
                    except CryptoError as exc:
                        raise VaultError("Failed to decrypt file (stream authentication failed).") from exc
            elif format_txt == FORMAT_V3:
                enc_data = z.read(source_name)
                if len(enc_data) < (NONCE_SIZE * 2):
                    raise VaultError("Corrupted file data.")
                c_n = enc_data[:NONCE_SIZE]
                a_n = enc_data[NONCE_SIZE:(NONCE_SIZE*2)]
                f_cipher = enc_data[(NONCE_SIZE*2):]
                try:
                    target.write(decrypt_data_v3(self.key, c_n, a_n, f_cipher))
                except CryptoError:
                    raise VaultError("Failed to decrypt file (Cascade Layer failed).")
            else:
                enc_data = z.read(source_name)
                if len(enc_data) < NONCE_SIZE:
                    raise VaultError("Corrupted file data.")
                f_nonce = enc_data[:NONCE_SIZE]
                f_cipher = enc_data[NONCE_SIZE:]
                try:
                    target.write(decrypt_data(self.key, f_nonce, f_cipher))
                except CryptoError:
                    raise VaultError("Failed to decrypt file.")

    def add_file(
        self,
        file_path: Path,
        overwrite: bool = False,
        progress_cb: Callable[[int, int], None] = None,
        vault_name: str = None,
    ):
        if not file_path.exists() or not file_path.is_file():
            raise VaultError("Selected path is not a file.")

        filename = safe_filename(vault_name or file_path.name)
        files = self.data.setdefault("files", {})

        if filename in files and not overwrite:
            raise VaultError(f"'{filename}' already exists in the vault.")

        file_size = file_path.stat().st_size
        internal_id = str(uuid.uuid4())

        file_hash = stream_sha256(file_path)

        if progress_cb:
            progress_cb(file_size, file_size)

        old_meta = files.get(filename)
        files[filename] = {
            "name": filename,
            "size": file_size,
            "sha256": file_hash,
            "added_at": now_unix(),
            "updated_at": now_unix(),
            "type": "file",
            "internal_id": internal_id
        }

        try:
            def write_entries(z):
                if self.vault_path.exists() and zipfile.is_zipfile(self.vault_path):
                    with checked_zip(self.vault_path) as old_z:
                        for item in old_z.infolist():
                            if not item.filename.startswith("data/"):
                                continue
                            if item.filename == f"data/{internal_id}.enc":
                                continue
                            keep = any(
                                meta.get("internal_id") and item.filename == f"data/{meta['internal_id']}.enc"
                                for name, meta in self.data["files"].items()
                                if name != filename
                            )
                            if keep:
                                with old_z.open(item.filename, "r") as source:
                                    with z.open(item, "w") as target:
                                        shutil.copyfileobj(source, target)

                with open(file_path, "rb") as source:
                    with z.open(f"data/{internal_id}.enc", "w") as target:
                        encrypt_stream_v5(self.key, source, target, compress=True)

            self._write_vault_zip(self.salt, self.key, write_entries)
        except Exception:
            if old_meta is None:
                files.pop(filename, None)
            else:
                files[filename] = old_meta
            raise

    def _update_metadata_only(self):
        """Alias for save(); ZIP updates require a full rewrite."""
        self.save()

    def add_folder_as_zip(
        self,
        folder_path: Path,
        overwrite: bool = False,
        progress_cb: Callable[[int, int], None] = None,
        max_files: int = MAX_FOLDER_FILES,
        max_bytes: int = MAX_FOLDER_BYTES,
    ):
        if not folder_path.exists() or not folder_path.is_dir():
            raise VaultError("Selected path is not a folder.")

        zip_name = safe_filename(folder_path.name.rstrip("/").rstrip("\\") + ".zip")

        tmp_zip = None
        try:
            tmp_zip, tmp_zip_file = make_private_temp(self.vault_path, ".folder.zip")
            file_count = 0
            total_bytes = 0
            with tmp_zip_file:
                # The folder ZIP is plaintext until it is added to the vault, so
                # it lives in a private temp file and is wiped in the finally block.
                with zipfile.ZipFile(tmp_zip_file, "w", zipfile.ZIP_DEFLATED) as z:
                    for root, _, files in os.walk(folder_path):
                        for file in files:
                            full_path = Path(root) / file
                            if full_path.is_symlink() or not full_path.is_file():
                                continue
                            try:
                                size = full_path.stat().st_size
                            except OSError as exc:
                                raise VaultError(f"Could not read folder item: {full_path.name}") from exc

                            file_count += 1
                            total_bytes += size
                            if file_count > max_files:
                                raise VaultError(f"Folder import limit exceeded ({max_files} files).")
                            if total_bytes > max_bytes:
                                raise VaultError("Folder import is too large.")

                            try:
                                archive_name = full_path.relative_to(folder_path.parent)
                            except ValueError:
                                archive_name = full_path.name
                            z.write(full_path, archive_name)

            self.add_file(tmp_zip, overwrite=overwrite, progress_cb=progress_cb, vault_name=zip_name)
        finally:
            if tmp_zip:
                secure_unlink(tmp_zip)

        self.data["files"][zip_name]["type"] = "folder_zip"
        self._update_metadata_only()

    def extract_file(self, filename: str, output_dir: Path, overwrite: bool = False, progress_cb: Callable[[int, int], None] = None) -> Path:
        item = self.get_file_meta(filename)
        output_dir.mkdir(parents=True, exist_ok=True)
        output_path = output_dir / safe_filename(filename)

        ensure_not_symlink(output_path, output_path.name)
        if output_path.exists() and not overwrite:
            raise VaultError(f"'{output_path.name}' already exists in the output folder.")

        tmp_output = None

        try:
            tmp_output, target = make_private_temp(output_path, ".part")
            with target:
                self._decrypt_file_to_target(item, target)

            if progress_cb:
                progress_cb(item.get("size", 0), item.get("size", 0))

            expected_hash = item.get("sha256")
            if expected_hash and expected_hash != "skipped_large_file":
                if stream_sha256(tmp_output) != expected_hash:
                    raise VaultError("Extracted file hash mismatch. Output was removed.")

            ensure_not_symlink(output_path, output_path.name)
            tmp_output.replace(output_path)
        except Exception:
            if tmp_output:
                secure_unlink(tmp_output)
            raise

        return output_path

    def verify_file(self, filename: str) -> Dict[str, Any]:
        item = self.get_file_meta(filename)
        writer = HashWriter()
        self._decrypt_file_to_target(item, writer)

        expected_hash = item.get("sha256")
        actual_hash = writer.hexdigest()
        hash_checked = bool(expected_hash and expected_hash != "skipped_large_file")
        hash_ok = True if not hash_checked else actual_hash == expected_hash
        size_ok = writer.size == int(item.get("size", writer.size))

        if not hash_ok:
            raise VaultError(f"Hash mismatch for '{filename}'.")
        if not size_ok:
            raise VaultError(f"Size mismatch for '{filename}'.")

        return {
            "name": filename,
            "size": writer.size,
            "sha256": actual_hash,
            "hash_checked": hash_checked,
        }

    def verify_all(self, progress_cb: Callable[[int, int], None] = None) -> Dict[str, Any]:
        filenames = self.list_files()
        verified = []
        for index, filename in enumerate(filenames, start=1):
            verified.append(self.verify_file(filename))
            if progress_cb:
                progress_cb(index, len(filenames))

        return {
            "file_count": len(verified),
            "bytes_checked": sum(item["size"] for item in verified),
            "hash_checked_count": sum(1 for item in verified if item["hash_checked"]),
        }

    def delete_file(self, filename: str):
        files = self.data.get("files", {})
        if filename not in files:
            raise VaultError("File not found in vault.")
        del files[filename]
        self.save()

    def rename_file(self, old_name: str, new_name: str):
        files = self.data.get("files", {})
        if old_name not in files:
            raise VaultError("File not found in vault.")
        new_name = safe_filename(new_name)
        if new_name in files:
            raise VaultError("A file with that name already exists in the vault.")

        files[new_name] = files.pop(old_name)
        files[new_name]["name"] = new_name
        files[new_name]["updated_at"] = now_unix()
        self._update_metadata_only()

    def change_password(self, old_password: str, new_password: str):
        if not self._password_matches_current_key(old_password):
            raise VaultError("Current password is incorrect.")

        new_salt = os.urandom(SALT_SIZE)
        new_key = derive_key_scrypt(new_password, new_salt, self.kdf_n, self.kdf_r, self.kdf_p)
        old_data = copy.deepcopy(self.data)
        old_version = self.version

        old_key = self.key
        if old_key is None:
            raise VaultError("Vault is locked.")

        try:
            def write_entries(new_z):
                if zipfile.is_zipfile(self.vault_path):
                    with checked_zip(self.vault_path) as old_z:
                        format_txt = read_zip_entry(old_z, "format.txt", MAX_FORMAT_SIZE) if "format.txt" in old_z.namelist() else b""
                        for meta in self.data.get("files", {}).values():
                            internal_id = meta.get("internal_id")
                            if not internal_id:
                                continue

                            source_name = f"data/{internal_id}.enc"
                            if source_name not in old_z.namelist():
                                raise VaultError(f"Internal data file missing from vault: {internal_id}")

                            if format_txt == FORMAT_V5:
                                def decrypt_to_writer(writer, name=source_name):
                                    with old_z.open(name, "r") as source:
                                        decrypt_stream_v5(old_key, source, writer)
                            elif format_txt == FORMAT_V4:
                                def decrypt_to_writer(writer, name=source_name):
                                    with old_z.open(name, "r") as source:
                                        decrypt_stream_v4(old_key, source, writer)
                            elif format_txt == FORMAT_V3:
                                enc_data = old_z.read(source_name)
                                def decrypt_to_writer(writer, data=enc_data):
                                    if len(data) < (NONCE_SIZE * 2):
                                        raise VaultError("Corrupted file data.")
                                    c_n = data[:NONCE_SIZE]
                                    a_n = data[NONCE_SIZE:(NONCE_SIZE*2)]
                                    f_cipher = data[(NONCE_SIZE*2):]
                                    writer.write(decrypt_data_v3(old_key, c_n, a_n, f_cipher))
                            else:
                                enc_data = old_z.read(source_name)
                                def decrypt_to_writer(writer, data=enc_data):
                                    if len(data) < NONCE_SIZE:
                                        raise VaultError("Corrupted file data.")
                                    f_nonce = data[:NONCE_SIZE]
                                    f_cipher = data[NONCE_SIZE:]
                                    writer.write(decrypt_data(old_key, f_nonce, f_cipher))

                            with new_z.open(source_name, "w") as target:
                                encrypt_from_decrypting_source(decrypt_to_writer, new_key, target)

                for meta in self.data.get("files", {}).values():
                    if "content" not in meta:
                        continue

                    internal_id = str(uuid.uuid4())
                    meta["internal_id"] = internal_id
                    content_bytes = b64d(meta.pop("content"))
                    with new_z.open(f"data/{internal_id}.enc", "w") as target:
                        encrypt_stream_v5(new_key, BytesReader(content_bytes), target)

            self.data["updated_at"] = now_unix()
            self.data["version"] = 5
            self._write_vault_zip(new_salt, new_key, write_entries)
        except Exception:
            self.data = old_data
            self.version = old_version
            raise

        self.salt = new_salt
        self.key = new_key
        self.version = 5

    def stats(self) -> Dict[str, Any]:
        files = self.data.get("files", {})
        total_size = sum(int(item.get("size", 0)) for item in files.values())
        disk_size = self.vault_path.stat().st_size if self.vault_path.exists() else 0

        return {
            "file_count": len(files),
            "total_plain_size": total_size,
            "vault_disk_size": disk_size,
            "path": str(self.vault_path),
        }
