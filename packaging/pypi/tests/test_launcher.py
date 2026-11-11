import hashlib
import io
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock
from contextlib import redirect_stderr, redirect_stdout

# Keep the package tests runnable from the repository root as well as from
# the CI working directory, which supplies PYTHONPATH=src.
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from pulse_vault.launcher import (
    LauncherError,
    asset_name,
    asset_url,
    cached_binary,
    dispatch_launcher,
    main,
    parse_sha256sums,
    release_has_cli,
    sha256_file,
    tag_candidates,
)


class AssetNameTests(unittest.TestCase):
    def test_windows_amd64(self):
        self.assertEqual(asset_name("Windows", "AMD64"), "pulse-vault-windows-amd64.exe")

    def test_linux_arm64(self):
        self.assertEqual(asset_name("Linux", "aarch64"), "pulse-vault-linux-arm64")

    def test_darwin_x86(self):
        self.assertEqual(asset_name("Darwin", "x86_64"), "pulse-vault-darwin-amd64")

    def test_unknown_os(self):
        with self.assertRaises(LauncherError):
            asset_name("Plan9", "amd64")

    def test_unknown_cpu(self):
        with self.assertRaises(LauncherError):
            asset_name("Linux", "mips")


class ChecksumTests(unittest.TestCase):
    def test_parse_gnu_style(self):
        text = (
            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  pulse-vault-linux-amd64\n"
            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa *pulse-vault-windows-amd64.exe\n"
            "# comment\n"
        )
        got = parse_sha256sums(text)
        self.assertEqual(
            got["pulse-vault-linux-amd64"],
            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
        )
        self.assertEqual(got["pulse-vault-windows-amd64.exe"].count("a"), 64)

    def test_sha256_file(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "blob"
            p.write_bytes(b"pulse-vault")
            self.assertEqual(sha256_file(p), hashlib.sha256(b"pulse-vault").hexdigest())


class ReleasePickTests(unittest.TestCase):
    def test_tag_candidates(self):
        self.assertEqual(tag_candidates("0.2.0")[:3], ["0.2.0", "v0.2.0", "V0.2.0"])
        self.assertIn("v0.2.0", tag_candidates("v0.2.0"))
        self.assertEqual(tag_candidates(""), [])

    def test_release_has_cli(self):
        self.assertFalse(release_has_cli({"assets": []}))
        self.assertFalse(release_has_cli({"tag_name": "upload", "assets": []}))
        self.assertTrue(release_has_cli({"assets": [{"name": "SHA256SUMS"}]}))
        self.assertTrue(
            release_has_cli({"assets": [{"name": "pulse-vault-linux-amd64"}]})
        )

    def test_asset_url_prefers_browser_link(self):
        rel = {
            "tag_name": "v0.2.0",
            "assets": [
                {
                    "name": "SHA256SUMS",
                    "browser_download_url": "https://example.invalid/SHA256SUMS",
                }
            ],
        }
        self.assertEqual(asset_url(rel, "SHA256SUMS"), "https://example.invalid/SHA256SUMS")

    def test_fetch_release_explains_empty_latest(self):
        empty = {"tag_name": "upload", "name": "V0.2.0", "assets": []}
        with mock.patch("pulse_vault.launcher.github_json", side_effect=[empty, [empty]]):
            with self.assertRaises(LauncherError) as ctx:
                from pulse_vault.launcher import fetch_release

                fetch_release()
        self.assertIn("upload", str(ctx.exception))
        self.assertIn("v0.2.0", str(ctx.exception))


class LauncherCommandTests(unittest.TestCase):
    def test_help_and_version(self):
        self.assertEqual(dispatch_launcher("--launcher-help"), 0)
        self.assertEqual(dispatch_launcher("--launcher-version"), 0)

    def test_normal_cli_command_does_not_contact_github_when_cache_is_empty(self):
        with tempfile.TemporaryDirectory() as td:
            with mock.patch("pulse_vault.launcher.cache_dir", return_value=Path(td)), \
                    mock.patch("pulse_vault.launcher.asset_name", return_value="pulse-vault-test"), \
                    mock.patch("pulse_vault.launcher.fetch_release") as fetch, \
                    redirect_stderr(io.StringIO()):
                self.assertEqual(main(["version"]), 2)
            fetch.assert_not_called()

    def test_launcher_install_is_an_explicit_download_action(self):
        binary = Path("pulse-vault-test")
        with mock.patch("pulse_vault.launcher.ensure_binary", return_value=binary) as install, \
                mock.patch("pulse_vault.launcher.print_info"), \
                redirect_stdout(io.StringIO()):
            self.assertEqual(dispatch_launcher("--launcher-install"), 0)
        install.assert_called_once_with()

    def test_launcher_update_is_an_explicit_forced_download_action(self):
        binary = Path("pulse-vault-test")
        with mock.patch("pulse_vault.launcher.ensure_binary", return_value=binary) as update, \
                mock.patch("pulse_vault.launcher.print_info"), \
                redirect_stdout(io.StringIO()):
            self.assertEqual(dispatch_launcher("--launcher-update"), 0)
        update.assert_called_once_with(force=True)

    def test_verified_cached_binary_is_used_offline(self):
        with tempfile.TemporaryDirectory() as td:
            binary = Path(td) / "pulse-vault-test"
            binary.write_bytes(b"verified test binary")
            digest = hashlib.sha256(binary.read_bytes()).hexdigest()
            binary.with_suffix(binary.suffix + ".meta.json").write_text(
                '{"tag":"v1.2.3","sha256":"' + digest + '"}', encoding="utf-8"
            )
            with mock.patch("pulse_vault.launcher.cache_dir", return_value=Path(td)), \
                    mock.patch("pulse_vault.launcher.asset_name", return_value=binary.name), \
                    mock.patch("pulse_vault.launcher.fetch_release") as fetch:
                self.assertEqual(cached_binary(), binary)
            fetch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
