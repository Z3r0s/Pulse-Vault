"""Download (once) and exec the official Pulse-Vault Go CLI."""

from __future__ import annotations

import hashlib
import json
import os
import platform
import stat
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any
from urllib.parse import quote

from pulse_vault import HOMEPAGE, REPO, __version__

CACHE_ENV = "PULSE_VAULT_BIN"
VERSION_ENV = "PULSE_VAULT_VERSION"
REPO_ENV = "PULSE_VAULT_REPO"
UA = f"pulse-vault-pypi-launcher/{__version__} (+{HOMEPAGE})"


class LauncherError(RuntimeError):
    pass


def repo_name() -> str:
    return os.environ.get(REPO_ENV, "").strip() or REPO


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


def meta_path(dest: Path) -> Path:
    return dest.with_suffix(dest.suffix + ".meta.json")


def read_meta(dest: Path) -> dict[str, Any]:
    path = meta_path(dest)
    if not path.is_file():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}


def write_meta(dest: Path, data: dict[str, Any]) -> None:
    meta_path(dest).write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def tag_candidates(raw: str) -> list[str]:
    raw = raw.strip()
    if not raw:
        return []
    body = raw[1:] if raw[:1] in "vV" and len(raw) > 1 and raw[1].isdigit() else raw
    out: list[str] = []
    for t in (raw, f"v{body}", f"V{body}", body):
        if t and t not in out:
            out.append(t)
    return out


def _request(url: str, accept: str = "*/*") -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept": accept})
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return resp.read()
    except urllib.error.HTTPError as exc:
        raise LauncherError(f"HTTP {exc.code} for {url}") from exc
    except urllib.error.URLError as exc:
        raise LauncherError(f"download failed: {url}: {exc}") from exc


def github_json(url: str) -> Any:
    data = _request(url, accept="application/vnd.github+json")
    try:
        return json.loads(data.decode("utf-8"))
    except json.JSONDecodeError as exc:
        raise LauncherError(f"bad JSON from {url}") from exc


def release_has_cli(rel: dict[str, Any]) -> bool:
    for asset in rel.get("assets") or []:
        name = str(asset.get("name") or "")
        if name == "SHA256SUMS" or name.startswith("pulse-vault-"):
            return True
    return False


def fetch_release(want: str | None = None) -> dict[str, Any]:
    repo = repo_name()
    api = f"https://api.github.com/repos/{repo}/releases"
    last_err: Exception | None = None
    if want:
        for tag in tag_candidates(want):
            try:
                rel = github_json(f"{api}/tags/{quote(tag)}")
                if release_has_cli(rel):
                    return rel
                last_err = LauncherError(
                    f"GitHub release {tag!r} exists but has no CLI assets (only source zip?). "
                    "Push a lowercase v0.2.0 tag so Actions can attach binaries, or run "
                    "Actions → Release Go → Run workflow."
                )
            except LauncherError as exc:
                last_err = exc
        if last_err:
            raise last_err

    latest = github_json(f"{api}/latest")
    if release_has_cli(latest):
        return latest
    try:
        listing = github_json(f"{api}?per_page=15")
    except LauncherError:
        listing = []
    if isinstance(listing, list):
        for rel in listing:
            if isinstance(rel, dict) and release_has_cli(rel):
                return rel
    tag = latest.get("tag_name") or latest.get("name") or "?"
    raise LauncherError(
        f"GitHub latest release ({tag!r}) has no Pulse-Vault CLI files. "
        "Tag v0.2.0 (lowercase v) and push it so the Release Go workflow attaches "
        "pulse-vault-* and SHA256SUMS. https://github.com/"
        f"{repo}/releases"
    )


def asset_url(rel: dict[str, Any], name: str) -> str | None:
    for asset in rel.get("assets") or []:
        if asset.get("name") == name and asset.get("browser_download_url"):
            return str(asset["browser_download_url"])
    tag = rel.get("tag_name")
    if tag:
        return f"https://github.com/{repo_name()}/releases/download/{quote(str(tag))}/{name}"
    return None


def download_file(url: str, label: str) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            total = resp.headers.get("Content-Length")
            chunks: list[bytes] = []
            read = 0
            size = int(total) if total and total.isdigit() else 0
            while True:
                block = resp.read(256 * 1024)
                if not block:
                    break
                chunks.append(block)
                read += len(block)
                if size and sys.stderr.isatty():
                    pct = min(99, int(read * 100 / size))
                    sys.stderr.write(f"\r  downloading {label}  {pct}%")
                    sys.stderr.flush()
            if size and sys.stderr.isatty():
                sys.stderr.write(f"\r  downloading {label}  100%\n")
            return b"".join(chunks)
    except urllib.error.HTTPError as exc:
        raise LauncherError(f"HTTP {exc.code} downloading {label}") from exc
    except urllib.error.URLError as exc:
        raise LauncherError(f"download failed: {url}: {exc}") from exc


def install_from_release(rel: dict[str, Any], dest: Path, force: bool) -> Path:
    name = asset_name()
    dest.parent.mkdir(parents=True, exist_ok=True)
    sums_url = asset_url(rel, "SHA256SUMS")
    bin_url = asset_url(rel, name)
    if not sums_url or not bin_url:
        raise LauncherError(f"release is missing SHA256SUMS or {name}")
    sums_text = _request(sums_url).decode("utf-8", "replace")
    sums = parse_sha256sums(sums_text)
    if name not in sums:
        raise LauncherError(f"{name} is not listed in SHA256SUMS for this release")

    meta = read_meta(dest)
    if dest.is_file() and dest.stat().st_size > 0 and not force:
        if sha256_file(dest) == sums[name]:
            if meta.get("sha256") != sums[name] or meta.get("tag") != rel.get("tag_name"):
                write_meta(
                    dest,
                    {
                        "tag": rel.get("tag_name"),
                        "name": name,
                        "sha256": sums[name],
                    },
                )
            return dest
        sys.stderr.write("pulse-vault: cached CLI failed the checksum, re-downloading\n")

    sys.stderr.write(f"pulse-vault: fetching {name} from GitHub ({rel.get('tag_name')})\n")
    blob = download_file(bin_url, name)
    digest = hashlib.sha256(blob).hexdigest()
    if digest != sums[name]:
        raise LauncherError(f"SHA-256 mismatch for {name}: got {digest}, want {sums[name]}")
    tmp = dest.with_name(dest.name + ".part")
    tmp.write_bytes(blob)
    tmp.replace(dest)
    if os.name != "nt":
        dest.chmod(dest.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    write_meta(
        dest,
        {"tag": rel.get("tag_name"), "name": name, "sha256": digest},
    )
    return dest


def ensure_binary(*, force: bool = False) -> Path:
    override = os.environ.get(CACHE_ENV, "").strip()
    if override:
        path = Path(override)
        if not path.is_file():
            raise LauncherError(f"{CACHE_ENV}={override} is not a file")
        return path

    dest = cache_dir() / asset_name()
    want = os.environ.get(VERSION_ENV, "").strip() or None
    if dest.is_file() and dest.stat().st_size > 0 and not force and not want:
        meta = read_meta(dest)
        if meta.get("sha256") and sha256_file(dest) == meta["sha256"]:
            return dest
    rel = fetch_release(want)
    return install_from_release(rel, dest, force=force)


def launcher_help() -> str:
    return f"""Pulse-Vault pip launcher {__version__}  ({HOMEPAGE})

This command is a small Python wrapper. It downloads the official Go CLI
from GitHub Releases, checks SHA256SUMS, then runs that binary.

  pulse-vault ...                  run the Go CLI
  pulse-vault --launcher-version   wrapper version
  pulse-vault --launcher-which     path of the cached Go binary
  pulse-vault --launcher-info      tag, hash, asset name
  pulse-vault --launcher-update    re-download the latest CLI
  pulse-vault --launcher-help      this text

Env:
  {CACHE_ENV}       use this binary, skip GitHub
  {VERSION_ENV}   pin a tag (v0.2.0 / 0.2.0)
  {REPO_ENV}      GitHub repo (default {REPO})
"""


def print_info(path: Path) -> None:
    meta = read_meta(path)
    sys.stdout.write(f"launcher  {__version__}\n")
    sys.stdout.write(f"binary    {path}\n")
    sys.stdout.write(f"tag       {meta.get('tag') or '(unknown)'}\n")
    sys.stdout.write(f"asset     {meta.get('name') or path.name}\n")
    sys.stdout.write(f"sha256    {meta.get('sha256') or sha256_file(path)}\n")
    sys.stdout.write(f"site      {HOMEPAGE}\n")


def dispatch_launcher(cmd: str) -> int:
    if cmd == "--launcher-help":
        sys.stdout.write(launcher_help())
        return 0
    if cmd == "--launcher-version":
        sys.stdout.write(f"pulse-vault pip launcher {__version__} ({HOMEPAGE})\n")
        return 0
    try:
        if cmd == "--launcher-update":
            path = ensure_binary(force=True)
            sys.stdout.write(f"updated {path}\n")
            print_info(path)
            return 0
        path = ensure_binary()
        if cmd == "--launcher-which":
            sys.stdout.write(f"{path}\n")
            return 0
        if cmd == "--launcher-info":
            print_info(path)
            return 0
    except LauncherError as exc:
        sys.stderr.write(f"pulse-vault: {exc}\n")
        sys.stderr.write(f"Site: {HOMEPAGE}\n")
        return 2
    return 0


def run_binary(path: Path, args: list[str]) -> int:
    argv = [str(path), *args]
    if os.name == "nt":
        return subprocess.call(argv)
    os.execv(str(path), argv)
    return 0


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    if args and args[0].startswith("--launcher-"):
        if len(args) != 1:
            sys.stderr.write("pulse-vault: launcher flags take no extra arguments\n")
            return 2
        return dispatch_launcher(args[0])
    try:
        binary = ensure_binary()
    except LauncherError as exc:
        sys.stderr.write(f"pulse-vault: {exc}\n")
        sys.stderr.write(f"Site: {HOMEPAGE}\n")
        return 2
    return run_binary(binary, args)


if __name__ == "__main__":
    raise SystemExit(main())
