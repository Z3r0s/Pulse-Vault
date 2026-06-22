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

    def test_app_can_be_constructed_headless(self):
        with mock.patch("pulsevault.gui.app.ctk.CTk"):
            from pulsevault.gui.app import VaultGUI

            app = VaultGUI()
            app.setup_drag_drop = mock.Mock()
            app.setup_drag_drop()
            self.assertIsNotNone(app)


if __name__ == "__main__":
    unittest.main()