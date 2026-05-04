import sys
import os

# Suppress VMware/LibEGL 3D acceleration warnings on Linux VMs
os.environ["LIBGL_ALWAYS_SOFTWARE"] = "1"
os.environ["GALLIUM_DRIVER"] = "llvmpipe"

import customtkinter as ctk
from gui.app import VaultGUI

def main():
    ctk.set_appearance_mode("System")
    ctk.set_default_color_theme("blue")
    
    app = VaultGUI()
    
    if len(sys.argv) > 1:
        app.auto_open_vault(sys.argv[1])
        
    app.mainloop()

if __name__ == "__main__":
    main()
