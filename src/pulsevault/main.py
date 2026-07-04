import os
import sys
import argparse

from pulsevault import __version__, __app_name__


def _parse_early_args(argv=None):
    """Parse a minimal set of args without triggering heavy GUI imports."""
    p = argparse.ArgumentParser(add_help=False)
    p.add_argument("--version", action="store_true")
    p.add_argument("--cli", action="store_true")
    p.add_argument("--help", "-h", action="store_true")
    # known early flags only; full parsing in cli or gui path
    known, _ = p.parse_known_args(argv)
    return known


def main(argv=None):
    if argv is None:
        argv = sys.argv[1:]

    early = _parse_early_args(argv)

    if early.version:
        print(f"{__app_name__} {__version__}")
        return 0

    # CLI / guided mode (no tkinter / GUI imports)
    if early.cli or "--cli" in argv or (len(argv) > 0 and argv[0] in ("cli", "--help", "-h", "create", "open", "list", "verify", "change-password")):
        # Lazy import to avoid pulling customtkinter / tk when doing pure CLI
        from pulsevault.cli import run_guided_cli
        # strip --cli / cli tokens for the subparser
        clean = [a for a in argv if a not in ("--cli", "cli")]
        return run_guided_cli(clean)

    if early.help:
        # Show combined basic help
        print(f"{__app_name__} v{__version__}")
        print("Usage:")
        print("  pulse-vault [VAULT]          Launch GUI (optionally auto-open vault)")
        print("  pulse-vault --cli [subcmd]   Guided terminal / CLI mode (good for pip/apt/snap)")
        print("  pulse-vault --version")
        print("  pulse-vault --help")
        print("\nIn CLI mode: create, open, list, verify, change-password")
        print("Example: pulse-vault --cli create myvault.pulsevault")
        return 0

    # Default / GUI path (original behavior)
    # Suppress VMware/LibEGL 3D acceleration warnings on Linux VMs.
    os.environ.setdefault("LIBGL_ALWAYS_SOFTWARE", "1")
    os.environ.setdefault("GALLIUM_DRIVER", "llvmpipe")

    import customtkinter as ctk
    from pulsevault.gui.app import VaultGUI

    ctk.set_appearance_mode("System")
    ctk.set_default_color_theme("blue")

    app = VaultGUI()
    if argv:
        # pass first non-flag arg as potential vault path (backward compat)
        candidate = next((a for a in argv if not a.startswith("-")), None)
        if candidate:
            app.auto_open_vault(candidate)
    app.mainloop()
    return 0


if __name__ == "__main__":
    sys.exit(main())
