import os
import json
import base64
import hashlib
import zipfile
import tempfile
import shutil
import time
import uuid
from pathlib import Path
from typing import Optional, Dict, Any, List, Callable

from core.crypto import (
    VAULT1_MAGIC,
    SALT_SIZE,
    NONCE_SIZE,
    derive_key,
    derive_key_v3,
    encrypt_data,
    decrypt_data,
    encrypt_data_v3,
    decrypt_data_v3,
    encrypt_stream_v4,
    decrypt_stream_v4,
    encrypt_stream_v5,
    decrypt_stream_v5,
    CryptoError
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

FORMAT_V5 = b"PULSEVAULT5_COMPRESSED_CASCADE"
FORMAT_V4 = b"PULSEVAULT4_CASCADE"
FORMAT_V3 = b"PULSEVAULT3_CASCADE"

def now_unix() -> int:
    return int(time.time())

def b64e(data: bytes) -> str:
    return base64.b64encode(data).decode("utf-8")

def b64d(data: str) -> bytes:
    return base64.b64decode(data.encode("utf-8"))

def safe_filename(name: str) -> str:
    name = name.strip().replace("\\", "_").replace("/", "_")
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

class EncryptedVault:
    def __init__(self, vault_path: Path):
        self.vault_path = vault_path
        self.password: Optional[str] = None
        self.salt: Optional[bytes] = None
        self.key: Optional[bytes] = None
        self.data: Dict[str, Any] = self.default_data()
        self.version = 5
        self.carrier_data: bytes = b"" # Stores steganography carrier image/video

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

    def create(self, password: str, carrier_path: Optional[Path] = None):
        if self.vault_path.exists():
            raise VaultError("A vault already exists at that location.")

        self.carrier_path = carrier_path if (carrier_path and carrier_path.exists()) else None

        self.password = password
        self.salt = os.urandom(SALT_SIZE)
        self.key = derive_key_v3(password, self.salt)
        self.data = self.default_data()
        self.version = 5
        self.save()

    def unlock(self, password: str):
        if not self.vault_path.exists():
            raise VaultError("Vault file does not exist.")

        # Read only the first MB to check for V1 or Carrier signatures
        with open(self.vault_path, "rb") as f:
            header_chunk = f.read(1024 * 1024 * 5) # Read up to 5MB for carrier probing
        
        # Check if legacy Z3R0VAULT1
        if header_chunk.startswith(VAULT1_MAGIC):
            # For V1, the file is usually small anyway, but we should read it safely.
            # V1 loads the whole JSON so we just read it.
            raw = self.vault_path.read_bytes()
            self._unlock_v1(password, raw)
            return

        # It's a ZIP container (V2 or V3)
        if not zipfile.is_zipfile(self.vault_path):
            raise VaultError("Invalid vault format. ZIP container corrupted.")

        with zipfile.ZipFile(self.vault_path, "r") as z:
            if "salt.bin" not in z.namelist() or "metadata.enc" not in z.namelist():
                raise VaultError("Invalid vault structure.")
                
            # Determine carrier size accurately using the offset of the first file
            infolist = z.infolist()
            if infolist:
                # Find the minimum header offset
                self.carrier_offset = min(info.header_offset for info in infolist)
            else:
                self.carrier_offset = 0
            
            # Check version marker to determine which crypto to use
            format_txt = b""
            if "format.txt" in z.namelist():
                format_txt = z.read("format.txt")
            is_v5 = format_txt == FORMAT_V5
            is_v4 = format_txt == FORMAT_V4
            is_v3 = format_txt == FORMAT_V3
            is_v2 = not is_v3 and not is_v4 and not is_v5
            
            with z.open("salt.bin") as f:
                salt = f.read()
                
            with z.open("metadata.enc") as f:
                enc_meta = f.read()

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
        key = derive_key_v3(password, salt)
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
        key = derive_key_v3(password, salt)
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
        key = derive_key_v3(password, salt)
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
            
        self.password = password
        self.salt = salt
        self.key = key
        self.data = loaded
        self.version = version
        if version < 5:
            self.change_password(password, password)

    def lock(self):
        self.password = None
        self.salt = None
        self.key = None
        self.data = self.default_data()

    def save(self):
        if not self.key or not self.salt:
            raise VaultError("Vault is locked.")

        self.data["updated_at"] = now_unix()
        self.data["version"] = 5
        self.version = 5

        with tempfile.NamedTemporaryFile(delete=False) as tf:
            temp_zip_path = Path(tf.name)

        with zipfile.ZipFile(temp_zip_path, "w", zipfile.ZIP_STORED) as z:
            z.writestr("salt.bin", self.salt)
            z.writestr("format.txt", FORMAT_V5)
            
            if self.vault_path.exists() and zipfile.is_zipfile(self.vault_path):
                # Copy existing data blocks
                with zipfile.ZipFile(self.vault_path, "r") as old_z:
                    for item in old_z.infolist():
                        if item.filename.startswith("data/"):
                            keep = False
                            for meta in self.data["files"].values():
                                if meta.get("internal_id") and item.filename == f"data/{meta['internal_id']}.enc":
                                    keep = True
                                    break
                            
                            if keep:
                                # Streaming copy block-by-block
                                with old_z.open(item.filename, "r") as source:
                                    with z.open(item, "w") as target:
                                        shutil.copyfileobj(source, target)
            
            # Upgrading from V1 -> V3 inline contents
            for fname, meta in list(self.data["files"].items()):
                if "content" in meta:
                    content_bytes = b64d(meta["content"])
                    internal_id = str(uuid.uuid4())
                    
                    with tempfile.NamedTemporaryFile(delete=False) as plain_tf:
                        plain_path = Path(plain_tf.name)
                        plain_tf.write(content_bytes)

                    try:
                        with open(plain_path, "rb") as source:
                            with z.open(f"data/{internal_id}.enc", "w") as target:
                                encrypt_stream_v5(self.key, source, target)
                    finally:
                        secure_unlink(plain_path)

                    meta["internal_id"] = internal_id
                    del meta["content"]

            plaintext = json.dumps(self.data, indent=2).encode("utf-8")
            c_nonce, a_nonce, ciphertext = encrypt_data_v3(self.key, plaintext)
            z.writestr("metadata.enc", c_nonce + a_nonce + ciphertext)

        tmp_path = self.vault_path.with_suffix(self.vault_path.suffix + ".tmp")
        with open(tmp_path, "wb") as out:
            # Write carrier data if present
            if getattr(self, "carrier_path", None) and self.carrier_path.exists():
                with open(self.carrier_path, "rb") as c_in:
                    shutil.copyfileobj(c_in, out)
            elif getattr(self, "carrier_offset", 0) > 0 and self.vault_path.exists():
                with open(self.vault_path, "rb") as c_in:
                    # Only read up to the carrier offset
                    bytes_left = self.carrier_offset
                    while bytes_left > 0:
                        chunk = c_in.read(min(bytes_left, 1024 * 1024 * 4)) # 4MB chunks
                        if not chunk: break
                        out.write(chunk)
                        bytes_left -= len(chunk)
                        
            # Write the ZIP file
            with open(temp_zip_path, "rb") as z_in:
                shutil.copyfileobj(z_in, out)
                
        temp_zip_path.unlink(missing_ok=True)
        tmp_path.replace(self.vault_path)

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

        with zipfile.ZipFile(self.vault_path, "r") as z:
            format_txt = z.read("format.txt") if "format.txt" in z.namelist() else b""
            source_name = f"data/{internal_id}.enc"
            if source_name not in z.namelist():
                raise VaultError("Internal data file missing from vault.")

            if format_txt == FORMAT_V5:
                with z.open(source_name, "r") as source:
                    decrypt_stream_v5(self.key, source, target)
            elif format_txt == FORMAT_V4:
                with z.open(source_name, "r") as source:
                    decrypt_stream_v4(self.key, source, target)
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

    def add_file(self, file_path: Path, overwrite: bool = False, progress_cb: Callable[[int, int], None] = None, skip_save: bool = False):
        if not file_path.exists() or not file_path.is_file():
            raise VaultError("Selected path is not a file.")

        filename = safe_filename(file_path.name)
        files = self.data.setdefault("files", {})

        if filename in files and not overwrite:
            raise VaultError(f"'{filename}' already exists in the vault.")

        file_size = file_path.stat().st_size
        internal_id = str(uuid.uuid4())

        # Stream SHA256 to avoid loading large files into RAM
        import hashlib
        if file_size < 1024 * 1024 * 512:  # Only hash files under 512MB
            h = hashlib.sha256()
            with open(file_path, "rb") as fh:
                while True:
                    blk = fh.read(1024 * 1024)
                    if not blk:
                        break
                    h.update(blk)
            file_hash = h.hexdigest()
        else:
            file_hash = "skipped_large_file"

        if progress_cb:
            progress_cb(file_size, file_size)

        files[filename] = {
            "name": filename,
            "size": file_size,
            "sha256": file_hash,
            "added_at": now_unix(),
            "updated_at": now_unix(),
            "type": "file",
            "internal_id": internal_id
        }

        if not self.vault_path.exists() or not zipfile.is_zipfile(self.vault_path):
            self.save()
        
        with zipfile.ZipFile(self.vault_path, "a", zipfile.ZIP_STORED) as z:
            with open(file_path, "rb") as source:
                with z.open(f"data/{internal_id}.enc", "w") as target:
                    encrypt_stream_v5(self.key, source, target, compress=True)
        
        if not skip_save:
            self.save()  # One atomic rewrite with updated metadata

    def _update_metadata_only(self):
        """Alias for save() — zip format requires full rewrite to update any entry."""
        self.save()

    def add_folder_as_zip(self, folder_path: Path, overwrite: bool = False, progress_cb: Callable[[int, int], None] = None):
        if not folder_path.exists() or not folder_path.is_dir():
            raise VaultError("Selected path is not a folder.")

        zip_name = safe_filename(folder_path.name.rstrip("/").rstrip("\\") + ".zip")

        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_zip = Path(tmpdir) / zip_name
            with zipfile.ZipFile(tmp_zip, "w", zipfile.ZIP_DEFLATED) as z:
                for root, _, files in os.walk(folder_path):
                    for file in files:
                        full_path = Path(root) / file
                        try:
                            archive_name = full_path.relative_to(folder_path.parent)
                        except ValueError:
                            archive_name = full_path.name
                        z.write(full_path, archive_name)
            
            # Avoid double-save by skipping the update inside add_file temporarily
            self.add_file(tmp_zip, overwrite=overwrite, progress_cb=progress_cb, skip_save=True)
            
        self.data["files"][zip_name]["type"] = "folder_zip"
        self._update_metadata_only()

    def extract_file(self, filename: str, output_dir: Path, overwrite: bool = False, progress_cb: Callable[[int, int], None] = None) -> Path:
        item = self.get_file_meta(filename)
        output_dir.mkdir(parents=True, exist_ok=True)
        output_path = output_dir / filename

        if output_path.exists() and not overwrite:
            raise VaultError(f"'{output_path.name}' already exists in the output folder.")

        with open(output_path, "wb") as target:
            self._decrypt_file_to_target(item, target)

        if progress_cb:
            progress_cb(item.get("size", 0), item.get("size", 0))

        expected_hash = item.get("sha256")
        if expected_hash and expected_hash != "skipped_large_file":
            import hashlib
            h = hashlib.sha256()
            with open(output_path, "rb") as f:
                while True:
                    chunk = f.read(1024 * 1024)
                    if not chunk:
                        break
                    h.update(chunk)
            if h.hexdigest() != expected_hash:
                try:
                    output_path.unlink()
                except Exception:
                    pass
                raise VaultError("Extracted file hash mismatch. Output was removed.")

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
        if old_password != self.password:
            raise VaultError("Current password is incorrect.")

        new_salt = os.urandom(SALT_SIZE)
        new_key = derive_key_v3(new_password, new_salt)

        old_key = self.key
        if old_key is None:
            raise VaultError("Vault is locked.")

        with tempfile.NamedTemporaryFile(delete=False) as tf:
            temp_zip_path = Path(tf.name)

        try:
            with zipfile.ZipFile(temp_zip_path, "w", zipfile.ZIP_STORED) as new_z:
                new_z.writestr("salt.bin", new_salt)
                new_z.writestr("format.txt", FORMAT_V5)

                if zipfile.is_zipfile(self.vault_path):
                    with zipfile.ZipFile(self.vault_path, "r") as old_z:
                        format_txt = old_z.read("format.txt") if "format.txt" in old_z.namelist() else b""
                        for meta in self.data.get("files", {}).values():
                            internal_id = meta.get("internal_id")
                            if not internal_id:
                                continue

                            source_name = f"data/{internal_id}.enc"
                            if source_name not in old_z.namelist():
                                raise VaultError(f"Internal data file missing from vault: {internal_id}")

                            with tempfile.NamedTemporaryFile(delete=False) as plain_tf:
                                plain_path = Path(plain_tf.name)

                            try:
                                if format_txt == FORMAT_V5:
                                    with old_z.open(source_name, "r") as source:
                                        with open(plain_path, "wb") as target:
                                            decrypt_stream_v5(old_key, source, target)
                                elif format_txt == FORMAT_V4:
                                    with old_z.open(source_name, "r") as source:
                                        with open(plain_path, "wb") as target:
                                            decrypt_stream_v4(old_key, source, target)
                                elif format_txt == FORMAT_V3:
                                    enc_data = old_z.read(source_name)
                                    if len(enc_data) < (NONCE_SIZE * 2):
                                        raise VaultError("Corrupted file data.")
                                    c_n = enc_data[:NONCE_SIZE]
                                    a_n = enc_data[NONCE_SIZE:(NONCE_SIZE*2)]
                                    f_cipher = enc_data[(NONCE_SIZE*2):]
                                    plaintext = decrypt_data_v3(old_key, c_n, a_n, f_cipher)
                                    plain_path.write_bytes(plaintext)
                                else:
                                    enc_data = old_z.read(source_name)
                                    if len(enc_data) < NONCE_SIZE:
                                        raise VaultError("Corrupted file data.")
                                    f_nonce = enc_data[:NONCE_SIZE]
                                    f_cipher = enc_data[NONCE_SIZE:]
                                    plaintext = decrypt_data(old_key, f_nonce, f_cipher)
                                    plain_path.write_bytes(plaintext)

                                with open(plain_path, "rb") as source:
                                    with new_z.open(source_name, "w") as target:
                                        encrypt_stream_v5(new_key, source, target)
                            finally:
                                secure_unlink(plain_path)

                for meta in self.data.get("files", {}).values():
                    if "content" not in meta:
                        continue

                    internal_id = str(uuid.uuid4())
                    meta["internal_id"] = internal_id
                    content_bytes = b64d(meta.pop("content"))

                    with tempfile.NamedTemporaryFile(delete=False) as plain_tf:
                        plain_path = Path(plain_tf.name)
                        plain_tf.write(content_bytes)

                    try:
                        with open(plain_path, "rb") as source:
                            with new_z.open(f"data/{internal_id}.enc", "w") as target:
                                encrypt_stream_v5(new_key, source, target)
                    finally:
                        secure_unlink(plain_path)

                self.data["updated_at"] = now_unix()
                self.data["version"] = 5
                plaintext_meta = json.dumps(self.data, indent=2).encode("utf-8")
                c_nonce, a_nonce, ciphertext_meta = encrypt_data_v3(new_key, plaintext_meta)
                new_z.writestr("metadata.enc", c_nonce + a_nonce + ciphertext_meta)

            tmp_path = self.vault_path.with_suffix(self.vault_path.suffix + ".tmp")
            with open(tmp_path, "wb") as out:
                if getattr(self, "carrier_path", None) and self.carrier_path.exists():
                    with open(self.carrier_path, "rb") as c_in:
                        shutil.copyfileobj(c_in, out)
                elif getattr(self, "carrier_offset", 0) > 0 and self.vault_path.exists():
                    with open(self.vault_path, "rb") as c_in:
                        bytes_left = self.carrier_offset
                        while bytes_left > 0:
                            chunk = c_in.read(min(bytes_left, 1024 * 1024 * 4))
                            if not chunk:
                                break
                            out.write(chunk)
                            bytes_left -= len(chunk)

                with open(temp_zip_path, "rb") as z_in:
                    shutil.copyfileobj(z_in, out)

            tmp_path.replace(self.vault_path)
        finally:
            temp_zip_path.unlink(missing_ok=True)

        self.salt = new_salt
        self.key = new_key
        self.password = new_password
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
