import unittest

from pulsevault.gui.theme import resolve_appearance_mode, tree_palette


class GuiThemeTests(unittest.TestCase):
    def test_tree_palette_has_dark_and_light(self):
        dark = tree_palette("dark")
        light = tree_palette("light")
        self.assertIn("bg", dark)
        self.assertIn("bg", light)
        self.assertNotEqual(dark["bg"], light["bg"])

    def test_resolve_appearance_mode_maps_ctk_values(self):
        self.assertEqual(resolve_appearance_mode("Light"), "light")
        self.assertEqual(resolve_appearance_mode("Dark"), "dark")


if __name__ == "__main__":
    unittest.main()