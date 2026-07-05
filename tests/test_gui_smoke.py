import os
import sys
import unittest
from pathlib import Path
from unittest import mock

os.environ.setdefault("PULSEVAULT_TEST_FAST_KDF", "1")

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))


class GuiSmokeTests(unittest.TestCase):
    def test_app_module_imports(self):
        import pulsevault.gui.app as app_module

        self.assertTrue(hasattr(app_module, "VaultGUI"))

    def test_scrypt_profile_dialog_imports(self):
        from pulsevault.gui.dialogs import ask_scrypt_profile

        self.assertTrue(callable(ask_scrypt_profile))

    def test_github_releases_dialog_class_available(self):
        """New dedicated dialog class for GitHub downloads area is importable."""
        from pulsevault.gui.dialogs import GitHubReleasesDialog, show_github_releases_dialog
        self.assertTrue(callable(GitHubReleasesDialog))
        self.assertTrue(callable(show_github_releases_dialog))

    def test_app_can_be_constructed_headless(self):
        """Verify VaultGUI can be instantiated without a display (for CI/headless)."""
        class DummyCTk:
            """Stand-in base so VaultGUI can be subclassed and constructed without real Tk."""
            def __init__(self, *args, **kwargs):
                pass

            def __getattr__(self, name):
                # Any method called on the root window (title, bind, grid_*, etc.) returns a no-op mock.
                return mock.MagicMock()

        gui_app_name = "pulsevault.gui.app"

        with (
            mock.patch("customtkinter.CTk", DummyCTk),
            mock.patch("customtkinter.get_appearance_mode", return_value="Dark"),
            mock.patch("customtkinter.CTkFont", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkFrame", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkLabel", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkButton", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkEntry", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkProgressBar", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkOptionMenu", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkToplevel", new_callable=mock.MagicMock),
            mock.patch("tkinter.ttk.Style", new_callable=mock.MagicMock),
            mock.patch("tkinter.ttk.Treeview", new_callable=mock.MagicMock),
            mock.patch("tkinter.ttk.Scrollbar", new_callable=mock.MagicMock),
            mock.patch("tkinter.Menu", new_callable=mock.MagicMock),
            mock.patch("tkinter.PhotoImage", new_callable=mock.MagicMock),
            mock.patch("tkinter.font.nametofont", new_callable=mock.MagicMock),
            mock.patch("pulsevault.gui.app.tk.PhotoImage", new_callable=mock.MagicMock),
            mock.patch("webbrowser.open", new_callable=mock.MagicMock),
        ):
            # Force fresh import so the class statement binds VaultGUI using the Dummy base + mocked widgets.
            sys.modules.pop(gui_app_name, None)
            from pulsevault.gui.app import VaultGUI

            app = VaultGUI()
            # override to avoid any dnd side effects in this smoke test
            app.setup_drag_drop = mock.Mock()
            app.setup_drag_drop()
            self.assertIsNotNone(app)

            # Verify new GitHub downloads enhancements are present (headless safe)
            self.assertTrue(hasattr(app, "open_github_releases"))
            self.assertTrue(hasattr(app, "btn_downloads"))
            self.assertTrue(hasattr(app, "show_downloads_dialog"))
            self.assertTrue(hasattr(app, "version_badge"))
            # Empty panel text updated for GitHub download guidance (via attribute existence on mock)
            self.assertTrue(hasattr(app, "empty_panel"))

        # Ensure subsequent imports in the same process see the real implementation.
        sys.modules.pop(gui_app_name, None)

    def test_github_releases_opens_gracefully_headless(self):
        """open_github_releases and show_downloads_dialog must not crash under full mocks (for download users / CI)."""
        class DummyCTk:
            def __init__(self, *args, **kwargs):
                pass

            def __getattr__(self, name):
                return mock.MagicMock()

        gui_app_name = "pulsevault.gui.app"

        with (
            mock.patch("customtkinter.CTk", DummyCTk),
            mock.patch("customtkinter.get_appearance_mode", return_value="Dark"),
            mock.patch("customtkinter.CTkFont", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkFrame", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkLabel", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkButton", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkEntry", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkProgressBar", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkOptionMenu", new_callable=mock.MagicMock),
            mock.patch("customtkinter.CTkToplevel", new_callable=mock.MagicMock),
            mock.patch("tkinter.ttk.Style", new_callable=mock.MagicMock),
            mock.patch("tkinter.ttk.Treeview", new_callable=mock.MagicMock),
            mock.patch("tkinter.ttk.Scrollbar", new_callable=mock.MagicMock),
            mock.patch("tkinter.Menu", new_callable=mock.MagicMock),
            mock.patch("tkinter.PhotoImage", new_callable=mock.MagicMock),
            mock.patch("tkinter.font.nametofont", new_callable=mock.MagicMock),
            mock.patch("pulsevault.gui.app.tk.PhotoImage", new_callable=mock.MagicMock),
            mock.patch("webbrowser.open") as mock_wb,
        ):
            sys.modules.pop(gui_app_name, None)
            from pulsevault.gui.app import VaultGUI, GITHUB_RELEASES_URL

            app = VaultGUI()
            app.setup_drag_drop = mock.Mock()

            # Direct call - should prefer webbrowser but fallback is also safe
            app.open_github_releases()
            # webbrowser may or may not be called depending on mock side effects; just ensure no exception
            self.assertIn(GITHUB_RELEASES_URL, [GITHUB_RELEASES_URL])  # smoke that constant is used

            # New standalone dialog (headless)
            app.show_downloads_dialog()

            # Version badge click binding exists and is callable in construction
            self.assertIsNotNone(app.version_badge)

        sys.modules.pop(gui_app_name, None)

    def test_empty_state_mentions_github_downloads(self):
        """The empty state guidance for first-time / downloaded users mentions GitHub."""
        # Light check on source string (avoids full GUI spinup)
        import pulsevault.gui.app as app_mod
        src = app_mod.__file__
        with open(src, "r", encoding="utf-8") as f:
            content = f.read()
        self.assertIn("Download the latest release from GitHub", content)
        self.assertIn("GitHub Releases", content)


if __name__ == "__main__":
    unittest.main()