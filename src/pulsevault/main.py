import os
import sys

import customtkinter as ctk

from pulsevault.gui.app import VaultGUI


def main():
    # Suppress VMware/LibEGL 3D acceleration warnings on Linux VMs.
    os.environ.setdefault("LIBGL_ALWAYS_SOFTWARE", "1")
    os.environ.setdefault("GALLIUM_DRIVER", "llvmpipe")

    ctk.set_appearance_mode("System")
    ctk.set_default_color_theme("blue")

    app = VaultGUI()
    if len(sys.argv) > 1:
        app.auto_open_vault(sys.argv[1])
    app.mainloop()
