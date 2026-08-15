"""Download (once) and exec the official Pulse-Vault Go CLI."""

from __future__ import annotations

import hashlib
import os
import platform
import stat
import sys
import urllib.error
import urllib.request
from pathlib import Path

REPO = "Z3r0s/Pulse-Vault"
CACHE_ENV = "PULSE_VAULT_BIN"
RELEASES = f"https://github.com/{REPO}/releases/latest/download"


class LauncherError(RuntimeError):
    pass


def asset_name(system: str | None = None, machine: str | None = None) -> str:
    system = (system or platform.system()).lower()
    machine = (machine or platform.machine()).lower()
    if machine in ("amd64", "x86_64"):
        arch = "amd64"
    elif machine in ("arm64", "aarch64"):
        arch = "arm64"
    else:
        raise LauncherError(f"unsupported CPU {machine!r}")
    if system.startswith("win"):
        return f"pulse-vault-windows-{arch}.exe"
    if system == "darwin":
        return f"pulse-vault-darwin-{arch}"
    if system == "linux":
        return f"pulse-vault-linux-{arch}"
    raise LauncherError(f"unsupported OS {system!r}")


def parse_sha256sums(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) < 2:
            continue
        digest, name = parts[0].lower(), parts[-1]
        name = name.lstrip("*")
        name = Path(name).name
        if len(digest) == 64 and all(c in "0123456789abcdef" for c in digest):
            out[name] = digest
    return out


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def cache_dir() -> Path:
    if sys.platform == "win32":
        root = Path(os.environ.get("LOCALAPPDATA", Path.home() / "AppData" / "Local"))
        return root / "Pulse-Vault" / "bin"
    xdg = os.environ.get("XDG_CACHE_HOME")
    root = Path(xdg) if xdg else Path.home() / ".cache"
    return root / "pulse-vault" / "bin"


def _download(url: str) -> bytes:
    req = urllib.request.Request(
        url,
        headers={"User-Agent": "pulse-vault-pypi-launcher/0.1 (+https://dnspulse.org)"},
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return resp.read()
    except urllib.error.URLError as exc:
        raise LauncherError(f"download failed: {url}: {exc}") from exc


def ensure_binary() -> Path:
    override = os.environ.get(CACHE_ENV, "").strip()
    if override:
        path = Path(override)
        if not path.is_file():
            raise LauncherError(f"{CACHE_ENV}={override} is not a file")
        return path

    name = asset_name()
    dest = cache_dir() / name
    if dest.is_file() and dest.stat().st_size > 0:
        return dest

    dest.parent.mkdir(parents=True, exist_ok=True)
    sums = parse_sha256sums(_download(f"{RELEASES}/SHA256SUMS").decode("utf-8", "replace"))
    if name not in sums:
        raise LauncherError(
            f"{name} is not in SHA256SUMS yet. Build from source or wait for a tagged GitHub Release. "
            "https://github.com/Z3r0s/Pulse-Vault/releases"
        )
    blob = _download(f"{RELEASES}/{name}")
    digest = hashlib.sha256(blob).hexdigest()
    if digest != sums[name]:
        raise LauncherError(f"SHA-256 mismatch for {name}: got {digest}, want {sums[name]}")
    tmp = dest.with_suffix(dest.suffix + ".part")
    tmp.write_bytes(blob)
    tmp.replace(dest)
    if os.name != "nt":
        dest.chmod(dest.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return dest


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    if args == ["--launcher-version"]:
        from pulse_vault import __version__, HOMEPAGE

        sys.stdout.write(f"pulse-vault pip launcher {__version__} ({HOMEPAGE})\n")
        return 0
    try:
        binary = ensure_binary()
    except LauncherError as exc:
        sys.stderr.write(f"pulse-vault: {exc}\n")
        sys.stderr.write("Site: https://dnspulse.org\n")
        return 2
    if os.name == "nt":
        raise SystemExit(os.spawnv(os.P_WAIT, str(binary), [str(binary), *args]))
    os.execv(str(binary), [str(binary), *args])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
