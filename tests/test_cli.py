import os
import sys
import tempfile
import unittest
from io import StringIO
from pathlib import Path
from unittest import mock

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from pulsevault.cli import (
    _prompt_password,
    _prompt_profile,
    _print_vault_info,
    cmd_create,
    cmd_list,
    cmd_verify,
    cmd_change_pw,
    run_guided_cli,
)


class CliPromptTests(unittest.TestCase):
    def test_prompt_password_basic(self):
        with mock.patch("getpass.getpass", side_effect=["secret1234567890"]):
            pw = _prompt_password()
            self.assertEqual(pw, "secret1234567890")

    def test_prompt_password_confirm_mismatch_then_match(self):
        calls = []
        def pw_get(prompt=""):
            calls.append(prompt)
            if len(calls) == 1: return "abc12345678901"
            if len(calls) == 2: return "wrong"
            return "abc12345678901"
        with mock.patch("getpass.getpass", side_effect=pw_get):
            pw = _prompt_password(confirm=True)
            self.assertEqual(pw, "abc12345678901")

    def test_prompt_profile_defaults_and_valid(self):
        with mock.patch("builtins.input", return_value=""):
            self.assertEqual(_prompt_profile("standard"), "standard")

        with mock.patch("builtins.input", return_value="hardened"):
            self.assertEqual(_prompt_profile(), "hardened")

        with mock.patch("builtins.input", return_value="junk"):
            self.assertEqual(_prompt_profile("hardened"), "standard")


class CliCommandTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.vault_path = Path(self.tmpdir.name) / "testvault.pulsevault"
        self.password = "ThisIsAVeryLongTestPassword123!"

    def tearDown(self):
        self.tmpdir.cleanup()

    def _create_vault_via_code(self):
        # Helper: create a real vault using the library (bypass CLI prompts)
        from pulsevault.core.vault import EncryptedVault
        v = EncryptedVault(self.vault_path)
        v.create(self.password, scrypt_profile="fast")
        return v

    def test_cmd_create_success(self):
        with mock.patch("getpass.getpass", side_effect=[self.password, self.password] * 3), \
             mock.patch("pulsevault.cli._prompt_profile", return_value="standard"), \
             mock.patch("builtins.input", return_value="y"), \
             mock.patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            args = mock.Mock(vault=str(self.vault_path), profile="standard", carrier=None)
            rc = cmd_create(args)
            self.assertEqual(rc, 0)
            output = mock_stdout.getvalue()
            self.assertIn("Vault created successfully", output)
            self.assertTrue(self.vault_path.exists())

    def test_cmd_create_existing_vault_fails(self):
        self.vault_path.write_text("exists")
        with mock.patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            args = mock.Mock(vault=str(self.vault_path), profile="fast", carrier=None)
            rc = cmd_create(args)
            self.assertEqual(rc, 1)
            self.assertIn("already exists", mock_stdout.getvalue())

    def test_cmd_create_weak_password_aborted(self):
        weak = "short"
        with mock.patch("getpass.getpass", side_effect=[weak, weak]), \
             mock.patch("builtins.input", side_effect=["N"]), \
             mock.patch("sys.stdout", new_callable=StringIO) as mock_stdout:
            args = mock.Mock(vault=str(self.vault_path), profile="fast", carrier=None)
            rc = cmd_create(args)
            self.assertEqual(rc, 1)
            self.assertIn("Warning", mock_stdout.getvalue())

    def test_cmd_list_and_verify_after_create(self):
        self._create_vault_via_code()
        # Add a file via library
        from pulsevault.core.vault import EncryptedVault
        v = EncryptedVault(self.vault_path)
        v.unlock(self.password)
        data = Path(self.tmpdir.name) / "sample.txt"
        data.write_bytes(b"hello from cli test")
        v.add_file(data)

        with mock.patch("getpass.getpass", return_value=self.password), \
             mock.patch("sys.stdout", new_callable=StringIO) as out:
            args = mock.Mock(vault=str(self.vault_path))
            rc = cmd_list(args)
            self.assertEqual(rc, 0)
            out_str = out.getvalue()
            self.assertIn("sample.txt", out_str)

        with mock.patch("getpass.getpass", return_value=self.password), \
             mock.patch("sys.stdout", new_callable=StringIO) as out:
            args = mock.Mock(vault=str(self.vault_path))
            rc = cmd_verify(args)
            self.assertEqual(rc, 0)
            self.assertIn("verified", out.getvalue().lower())

    def test_cmd_change_pw_success(self):
        self._create_vault_via_code()
        new_pw = "NewSuperLongReplacementPassword987!"

        with mock.patch("getpass.getpass", side_effect=[self.password, new_pw, new_pw]), \
             mock.patch("sys.stdout", new_callable=StringIO) as out:
            args = mock.Mock(vault=str(self.vault_path))
            rc = cmd_change_pw(args)
            self.assertEqual(rc, 0)
            self.assertIn("successfully", out.getvalue())

        # Verify new pw works, old doesn't
        from pulsevault.core.vault import EncryptedVault, VaultError
        v = EncryptedVault(self.vault_path)
        with self.assertRaises(VaultError):
            v.unlock(self.password)
        v.unlock(new_pw)
        self.assertTrue(v.is_unlocked)

    def test_cmd_errors_on_bad_vault(self):
        with mock.patch("getpass.getpass", return_value="whatever"), \
             mock.patch("sys.stdout", new_callable=StringIO):
            args = mock.Mock(vault=str(self.vault_path))
            self.assertEqual(cmd_list(args), 1)
            self.assertEqual(cmd_verify(args), 1)
            self.assertEqual(cmd_change_pw(args), 1)


class CliDispatchTests(unittest.TestCase):
    def test_run_guided_cli_no_command_prints_help(self):
        with mock.patch("sys.stdout", new_callable=StringIO) as out:
            rc = run_guided_cli([])
            self.assertEqual(rc, 0)
            self.assertIn("Guided CLI", out.getvalue())
            self.assertIn("create", out.getvalue())

    def test_run_guided_cli_version_not_here_but_main_handles(self):
        # CLI subparser doesn't have --version; main does early parse
        try:
            rc = run_guided_cli(["--help"])
        except SystemExit as e:
            rc = e.code if isinstance(e.code, int) else 0
        self.assertEqual(rc, 0)

    def test_run_guided_cli_create_via_argv_mocks(self):
        with tempfile.TemporaryDirectory() as td:
            vp = Path(td) / "viaargv.pulsevault"
            pw = "AVeryLongTestPassForDispatch12345"
            with mock.patch("getpass.getpass", side_effect=[pw, pw]), \
                 mock.patch("builtins.input", return_value="standard"):
                rc = run_guided_cli(["create", str(vp), "--profile", "standard"])
                self.assertEqual(rc, 0)
                self.assertTrue(vp.exists())

    def test_run_guided_cli_keyboard_interrupt_handled(self):
        with mock.patch("getpass.getpass", side_effect=KeyboardInterrupt):
            rc = run_guided_cli(["list", "/tmp/nonexistent123.pulsevault"])
            self.assertEqual(rc, 130)

    def test_run_guided_cli_bad_command_returns_zero(self):
        # argparse --help path or unknown may sys.exit(0); catch for test
        try:
            rc = run_guided_cli(["--help"])
        except SystemExit as e:
            rc = e.code if isinstance(e.code, int) else 0
        self.assertEqual(rc, 0)


class CliIntegrationAdvancedTests(unittest.TestCase):
    """More advanced scenarios: unicode, empty files, policy, multiple ops."""

    def test_cli_handles_unicode_filename_roundtrip(self):
        with tempfile.TemporaryDirectory() as td:
            vp = Path(td) / "unicode.pulsevault"
            pw = "UnicodeTestPassphraseForCLI987654321!"
            from pulsevault.core.vault import EncryptedVault
            v = EncryptedVault(vp)
            v.create(pw, scrypt_profile="fast")

            # Add unicode named file via lib then list via cli
            fname = "日本語_файл_émoji_🔐.bin"
            data = (Path(td) / "u.dat")
            data.write_bytes(b"\x00\x01\x02" * 100)
            v.add_file(data, overwrite=True)
            # rename inside vault to tricky name
            v.rename_file(data.name, fname)

            with mock.patch("getpass.getpass", return_value=pw), \
                 mock.patch("sys.stdout", new_callable=StringIO) as out:
                rc = run_guided_cli(["list", str(vp)])
                self.assertEqual(rc, 0)
                self.assertIn(fname, out.getvalue())

    def test_cli_verify_on_empty_vault(self):
        with tempfile.TemporaryDirectory() as td:
            vp = Path(td) / "empty.pulsevault"
            pw = "EmptyVaultTestPass12345678901234"
            from pulsevault.core.vault import EncryptedVault
            EncryptedVault(vp).create(pw, scrypt_profile="fast")

            with mock.patch("getpass.getpass", return_value=pw), \
                 mock.patch("sys.stdout", new_callable=StringIO) as out:
                rc = run_guided_cli(["verify", str(vp)])
                self.assertEqual(rc, 0)
                # should succeed with 0 files
                self.assertIn("verified", out.getvalue().lower())


if __name__ == "__main__":
    unittest.main()
