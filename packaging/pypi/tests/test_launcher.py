import hashlib
import unittest

from pulse_vault.launcher import asset_name, parse_sha256sums, sha256_file


class AssetNameTests(unittest.TestCase):
    def test_windows_amd64(self):
        self.assertEqual(asset_name("Windows", "AMD64"), "pulse-vault-windows-amd64.exe")

    def test_linux_arm64(self):
        self.assertEqual(asset_name("Linux", "aarch64"), "pulse-vault-linux-arm64")

    def test_darwin_x86(self):
        self.assertEqual(asset_name("Darwin", "x86_64"), "pulse-vault-darwin-amd64")

    def test_unknown_os(self):
        with self.assertRaises(Exception):
            asset_name("Plan9", "amd64")

    def test_unknown_cpu(self):
        with self.assertRaises(Exception):
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
        import tempfile
        from pathlib import Path

        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "blob"
            p.write_bytes(b"pulse-vault")
            self.assertEqual(sha256_file(p), hashlib.sha256(b"pulse-vault").hexdigest())


if __name__ == "__main__":
    unittest.main()
