"""Interop: Go-written vaults must unlock/extract via the Python V5 core.

Proves the Go product is the same on-disk format as the legacy Python oracle.
Requires `go` on PATH and a buildable repository Go module.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GUI_GO = ROOT / "gui-go"
sys.path.insert(0, str(ROOT / "src"))
sys.path.insert(0, str(ROOT / "tests"))

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")
os.environ.setdefault("PULSEVAULT_SCRYPT_PROFILE", "fast")


def _go_available() -> bool:
    return shutil.which("go") is not None and (ROOT / "go.mod").is_file()


@unittest.skipUnless(_go_available(), "Go toolchain / repository module not available")
class GoPythonInteropTests(unittest.TestCase):
    def test_go_create_python_unlock_extract(self):
        from pulsevault.core.vault import EncryptedVault

        with tempfile.TemporaryDirectory(prefix="pv_interop_") as td:
            td_path = Path(td)
            vault = td_path / "go-made.pulsevault"
            payload = td_path / "note.txt"
            payload.write_text("interop payload go→python", encoding="utf-8")
            pw = "InteropPassword123!"

            env = os.environ.copy()
            env["CGO_ENABLED"] = "0"
            env["GOCACHE"] = str(td_path / "go-cache")
            cmd = [
                "go",
                "run",
                "./cmd/pulse-vault",
                "create",
                str(vault),
                "--password",
                pw,
                "--profile",
                "fast",
            ]
            r = subprocess.run(cmd, cwd=str(GUI_GO), env=env, capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

            r = subprocess.run(
                [
                    "go",
                    "run",
                    "./cmd/pulse-vault",
                    "add",
                    str(vault),
                    "--password",
                    pw,
                    str(payload),
                ],
                cwd=str(GUI_GO),
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

            v = EncryptedVault(vault)
            v.unlock(pw)
            self.assertIn("note.txt", v.list_files())
            out = td_path / "out"
            out.mkdir()
            got = v.extract_file("note.txt", out, overwrite=True)
            self.assertEqual(got.read_text(encoding="utf-8"), "interop payload go→python")

    def test_python_create_go_unlock_list(self):
        from pulsevault.core.vault import EncryptedVault

        with tempfile.TemporaryDirectory(prefix="pv_interop2_") as td:
            td_path = Path(td)
            vault = td_path / "py-made.pulsevault"
            pw = "PythonThenGoPassword!"
            src = td_path / "secret.bin"
            data = b"python-oracle-payload\x00\x01"
            src.write_bytes(data)

            v = EncryptedVault(vault)
            v.create(pw, scrypt_profile="fast")
            v.add_file(src)

            env = os.environ.copy()
            env["CGO_ENABLED"] = "0"
            env["GOCACHE"] = str(td_path / "go-cache")
            r = subprocess.run(
                [
                    "go",
                    "run",
                    "./cmd/pulse-vault",
                    "list",
                    str(vault),
                    "--password",
                    pw,
                ],
                cwd=str(GUI_GO),
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
            self.assertIn("secret.bin", r.stdout)

            out_dir = td_path / "go-out"
            out_dir.mkdir()
            r = subprocess.run(
                [
                    "go",
                    "run",
                    "./cmd/pulse-vault",
                    "extract",
                    str(vault),
                    "--password",
                    pw,
                    "secret.bin",
                    str(out_dir),
                ],
                cwd=str(GUI_GO),
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
            self.assertEqual((out_dir / "secret.bin").read_bytes(), data)

    def test_go_unlocks_all_python_legacy_formats_and_migrates(self):
        from pulsevault.core.vault import EncryptedVault
        from vault_fixtures import (
            build_legacy_v1_vault,
            build_legacy_v2_vault,
            build_legacy_v3_vault,
            build_legacy_v4_vault,
        )

        with tempfile.TemporaryDirectory(prefix="pv_legacy_interop_") as td:
            td_path = Path(td)
            cli = td_path / ("pulse-vault.exe" if os.name == "nt" else "pulse-vault")
            env = os.environ.copy()
            env["CGO_ENABLED"] = "0"
            env["GOCACHE"] = str(td_path / "go-cache")
            built = subprocess.run(
                ["go", "build", "-o", str(cli), "./cmd/pulse-vault"],
                cwd=str(GUI_GO),
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(built.returncode, 0, built.stdout + built.stderr)

            builders = (
                ("v1", build_legacy_v1_vault, "legacy-v1-password-123!"),
                ("v2", build_legacy_v2_vault, "legacy-v2-password-123!"),
                ("v3", build_legacy_v3_vault, "legacy-v3-password-123!"),
                ("v4", build_legacy_v4_vault, "legacy-v4-password-123!"),
            )
            payload = b"legacy format interoperability payload\x00\x01"
            for label, builder, password in builders:
                vault_path = td_path / f"{label}.pulsevault"
                builder(vault_path, password, {f"{label}.txt": payload})
                listed = subprocess.run(
                    [str(cli), "list", str(vault_path), "--password", password],
                    env=env,
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(listed.returncode, 0, f"{label}: {listed.stdout}{listed.stderr}")
                self.assertIn(f"{label}.txt", listed.stdout)
                output = td_path / f"{label}-out"
                output.mkdir()
                extracted = subprocess.run(
                    [str(cli), "extract", str(vault_path), "--password", password, f"{label}.txt", str(output)],
                    env=env,
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(extracted.returncode, 0, f"{label}: {extracted.stdout}{extracted.stderr}")
                self.assertEqual((output / f"{label}.txt").read_bytes(), payload)

                migrated = subprocess.run(
                    [str(cli), "migrate", str(vault_path), "--password", password],
                    env=env,
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(migrated.returncode, 0, f"{label}: {migrated.stdout}{migrated.stderr}")
                current = EncryptedVault(vault_path)
                current.unlock(password)
                self.assertEqual(current.extract_file(f"{label}.txt", td_path / f"{label}-migrated").read_bytes(), payload)

    def test_go_carrier_picture_python_unlock(self):
        from pulsevault.core.vault import EncryptedVault

        # 1x1 transparent PNG; viewers accept trailing ZIP bytes after IEND.
        tiny_png = bytes(
            [
                0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
                0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
                0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
                0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
                0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
                0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
                0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
                0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
                0x42, 0x60, 0x82,
            ]
        )

        with tempfile.TemporaryDirectory(prefix="pv_carrier_interop_") as td:
            td_path = Path(td)
            cover = td_path / "cover.png"
            hidden = td_path / "hidden.png"
            payload = td_path / "secret.txt"
            data = b"hidden-in-picture-interop\x00\x01"
            cover.write_bytes(tiny_png)
            payload.write_bytes(data)
            pw = "CarrierInteropPassword123!"

            env = os.environ.copy()
            env["CGO_ENABLED"] = "0"
            env["GOCACHE"] = str(td_path / "go-cache")
            r = subprocess.run(
                [
                    "go",
                    "run",
                    "./cmd/pulse-vault",
                    "create",
                    str(hidden),
                    "--carrier",
                    str(cover),
                    "--password",
                    pw,
                    "--profile",
                    "fast",
                ],
                cwd=str(GUI_GO),
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

            r = subprocess.run(
                [
                    "go",
                    "run",
                    "./cmd/pulse-vault",
                    "add",
                    str(hidden),
                    "--password",
                    pw,
                    str(payload),
                ],
                cwd=str(GUI_GO),
                env=env,
                capture_output=True,
                text=True,
            )
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

            self.assertTrue(hidden.is_file(), f"expected picture vault at {hidden}")
            raw = hidden.read_bytes()
            self.assertTrue(raw.startswith(b"\x89PNG\r\n\x1a\n"))
            self.assertGreater(len(raw), len(tiny_png))

            v = EncryptedVault(hidden)
            v.unlock(pw)
            out = td_path / "out"
            out.mkdir()
            got = v.extract_file("secret.txt", out, overwrite=True)
            self.assertEqual(got.read_bytes(), data)


if __name__ == "__main__":
    unittest.main()
