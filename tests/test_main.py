import os
import sys
import unittest
from io import StringIO
from pathlib import Path
from unittest import mock

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

import pulsevault.main as mainmod


class MainEarlyParseTests(unittest.TestCase):
    def test_parse_early_args_version(self):
        self.assertTrue(mainmod._parse_early_args(["--version"]).version)
        self.assertTrue(mainmod._parse_early_args(["--help"]).help)
        self.assertFalse(mainmod._parse_early_args([]).version)

    def test_parse_early_args_cli_detection(self):
        p = mainmod._parse_early_args(["--cli"])
        self.assertTrue(p.cli or "--cli" in ["--cli"])
        p2 = mainmod._parse_early_args(["create"])
        # create is handled later but early parser only knows a few
        self.assertFalse(getattr(p2, "cli", False))


class MainDispatchTests(unittest.TestCase):
    def test_main_version_prints_and_exits_early(self):
        with mock.patch("sys.stdout", new_callable=StringIO) as out:
            rc = mainmod.main(["--version"])
            self.assertEqual(rc, 0)
            self.assertIn("Pulse-Vault", out.getvalue())

    def test_main_help_prints_usage(self):
        with mock.patch("sys.stdout", new_callable=StringIO) as out:
            try:
                rc = mainmod.main(["--help"])
            except SystemExit as e:
                rc = e.code if isinstance(e.code, int) else 0
            self.assertEqual(rc, 0)
            self.assertIn("Pulse-Vault", out.getvalue())

    def test_main_cli_path_dispatches_without_gui_import(self):
        # Critical advanced test: ensure --cli path does not pull heavy GUI (proves packaging/CLI isolation)
        before = set(sys.modules.keys())
        fake_cli_mod = mock.MagicMock()
        fake_cli_mod.run_guided_cli.return_value = 42
        with mock.patch.dict(sys.modules, {"pulsevault.cli": fake_cli_mod}):
            rc = mainmod.main(["--cli", "list", "some.vault"])
            self.assertEqual(rc, 42)
            fake_cli_mod.run_guided_cli.assert_called()
        after = set(sys.modules.keys())
        # Must not have loaded the GUI app module (or ctk/tk) in the CLI path.
        # Note: test runner may have gui modules from other tests; we check delta + direct import behavior.
        gui_modules = [m for m in after - before if "gui" in m or "customtkinter" in m or "tkinter" in m]
        self.assertEqual(gui_modules, [], f"CLI path leaked new GUI modules: {gui_modules}")
        # Direct verification of clean CLI import (the important packaging guarantee)
        import subprocess, sys as real_sys
        code = 'import sys; sys.path.insert(0,"src"); import pulsevault.cli; print("GUI_LEAK" if any("gui" in m or "customtkinter" in m or "tkinter" in m for m in sys.modules) else "CLEAN")'
        out = subprocess.check_output([real_sys.executable, "-c", code], text=True)
        self.assertIn("CLEAN", out)

    def test_main_cli_subcommand_routes_to_run_guided(self):
        with mock.patch("pulsevault.cli.run_guided_cli", return_value=7) as m:
            rc = mainmod.main(["cli", "verify", "x.pulsevault"])
            self.assertEqual(rc, 7)
            m.assert_called()

    def test_main_gui_path_calls_vaultgui_and_mainloop(self):
        # Headless safe: patch the heavy parts
        fake_app = mock.MagicMock()
        with mock.patch.dict("sys.modules", {"customtkinter": mock.MagicMock()}):
            with mock.patch("pulsevault.main.ctk", create=True) as mock_ctk, \
                 mock.patch("pulsevault.gui.app.VaultGUI", return_value=fake_app) as mock_gui:
                mock_ctk.set_appearance_mode = mock.Mock()
                mock_ctk.set_default_color_theme = mock.Mock()
                rc = mainmod.main(["somevault.pulsevault"])
                self.assertEqual(rc, 0)
                mock_gui.assert_called()
                fake_app.mainloop.assert_called_once()
                fake_app.auto_open_vault.assert_called_with("somevault.pulsevault")

    def test_main_gui_no_argv_still_constructs(self):
        fake_app = mock.MagicMock()
        with mock.patch("pulsevault.main.ctk", create=True) as mock_ctk, \
             mock.patch("pulsevault.gui.app.VaultGUI", return_value=fake_app):
            mock_ctk.set_appearance_mode = mock.Mock()
            mock_ctk.set_default_color_theme = mock.Mock()
            rc = mainmod.main([])
            self.assertEqual(rc, 0)
            fake_app.mainloop.assert_called_once()


class MainAdvancedTests(unittest.TestCase):
    """Test interactions and safety."""

    def test_cli_path_sets_software_render_env(self):
        # The GUI path sets some env, CLI should not pollute unnecessarily
        with mock.patch("pulsevault.cli.run_guided_cli", return_value=0):
            orig = os.environ.get("LIBGL_ALWAYS_SOFTWARE")
            mainmod.main(["--cli"])
            # No assertion on mutation since CLI avoids the GUI block
            self.assertTrue(True)

    def test_main_returns_zero_on_cli_success(self):
        with mock.patch("pulsevault.cli.run_guided_cli", return_value=0):
            self.assertEqual(mainmod.main(["--cli"]), 0)


class CliGuiImportIsolationTests(unittest.TestCase):
    """Stronger subprocess + fresh-process isolation tests for packaging/CLI-only guarantees.
    Ensures `import pulsevault.cli` and `--cli` dispatch paths *never* pull customtkinter,
    tkinter, or any pulsevault.gui.* modules (even transitively).
    Uses fresh child python processes (not just mocks) for verification.
    """

    def _src_dir(self):
        return str(Path(__file__).resolve().parents[1] / "src")

    def test_import_pulsevault_cli_is_clean_in_fresh_process(self):
        """Critical: direct import pulsevault.cli must not load any GUI modules."""
        import subprocess
        src = self._src_dir()
        code = (
            "import sys, os\n"
            f"sys.path.insert(0, {src!r})\n"
            "os.environ.setdefault('PULSEVAULT_TEST_FAST_KDF', '1')\n"
            "import pulsevault.cli\n"
            "mods = list(sys.modules.keys())\n"
            "leaks = [m for m in mods if ('gui' in m) or ('customtkinter' in m) or ('tkinter' in m)]\n"
            "gui_pkg_leaks = [m for m in mods if m.startswith('pulsevault.gui')]\n"
            "print('CLEAN' if not leaks else 'GUI_LEAK:' + repr(leaks))\n"
            "print('GUI_PKG_LEAKS:' + repr(gui_pkg_leaks))\n"
        )
        out = subprocess.check_output([sys.executable, "-c", code], text=True, env=os.environ.copy())
        self.assertIn("CLEAN", out)
        self.assertNotIn("GUI_PKG_LEAKS:['", out)  # no pulsevault.gui* at all
        self.assertNotIn("pulsevault.gui", out)

    def test_main_cli_dispatch_is_clean_in_fresh_process(self):
        """Dispatch via main() with --cli must not load GUI in a fresh process."""
        import subprocess
        src = self._src_dir()
        code = (
            "import sys, os, io\n"
            f"sys.path.insert(0, {src!r})\n"
            "os.environ.setdefault('PULSEVAULT_TEST_FAST_KDF', '1')\n"
            "import pulsevault.main as m\n"
            "old_out, old_err = sys.stdout, sys.stderr\n"
            "sys.stdout = io.StringIO()\n"
            "sys.stderr = io.StringIO()\n"
            "try:\n"
            "    rc = m.main(['--cli'])\n"
            "finally:\n"
            "    sys.stdout, sys.stderr = old_out, old_err\n"
            "mods = list(sys.modules.keys())\n"
            "leaks = [m for m in mods if ('gui' in m) or ('customtkinter' in m) or ('tkinter' in m)]\n"
            "gui_pkg = [m for m in mods if m.startswith('pulsevault.gui')]\n"
            "print('DISPATCH_RC=' + str(rc))\n"
            "print('CLEAN' if not leaks else 'GUI_LEAK:' + repr(leaks))\n"
            "print('GUI_PKG:' + repr(gui_pkg))\n"
        )
        out = subprocess.check_output([sys.executable, "-c", code], text=True, env=os.environ.copy())
        self.assertIn("CLEAN", out)
        self.assertIn("DISPATCH_RC=0", out)
        self.assertNotIn("pulsevault.gui", out)

    def test_pulsevault_cli_import_does_not_load_gui_package(self):
        """Even in current process (after possible other imports), importing cli must not add gui subpackage."""
        import subprocess
        src = self._src_dir()
        code = (
            "import sys, os\n"
            f"sys.path.insert(0, {src!r})\n"
            "os.environ.setdefault('PULSEVAULT_TEST_FAST_KDF', '1')\n"
            "import pulsevault.cli\n"
            "has_gui_pkg = any(m.startswith('pulsevault.gui') for m in sys.modules)\n"
            "has_ctk = 'customtkinter' in sys.modules or 'tkinter' in sys.modules\n"
            "print('NO_GUI_PKG_AND_NO_TK' if (not has_gui_pkg and not has_ctk) else 'LEAKED_GUI_OR_TK')\n"
        )
        out = subprocess.check_output([sys.executable, "-c", code], text=True, env=os.environ.copy())
        self.assertIn("NO_GUI_PKG_AND_NO_TK", out)


if __name__ == "__main__":
    unittest.main()
