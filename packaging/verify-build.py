#!/usr/bin/env python3
"""
Simple verification helper for Pulse-Vault builds.

Usage examples:
  python packaging/verify-build.py --help
  python packaging/verify-build.py --check-version
  python packaging/verify-build.py --cli-smoke
  python packaging/verify-build.py --gui-import-isolated   # only works if you didn't install [gui]

This script helps confirm that a built binary or source install behaves as expected
and that the CLI path stays clean (no accidental GUI imports).
"""

import argparse
import importlib
import subprocess
import sys
from pathlib import Path

def check_version():
    try:
        import pulsevault
        print(f"pulsevault version: {pulsevault.__version__}")
        return True
    except Exception as e:
        print(f"Failed to import pulsevault: {e}")
        return False

def cli_smoke():
    """Run a very basic non-interactive CLI command."""
    try:
        # Use the module entry so it works whether installed or via PYTHONPATH
        result = subprocess.run(
            [sys.executable, "-m", "pulsevault", "--cli", "--help"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        if result.returncode == 0 and "guided CLI" in result.stdout:
            print("CLI smoke test passed (help text looks good).")
            return True
        print("CLI smoke failed. stdout:", result.stdout[:500])
        print("stderr:", result.stderr[:500])
        return False
    except Exception as e:
        print(f"CLI smoke error: {e}")
        return False

def gui_import_isolated():
    """
    Confirm that importing the CLI does not pull in GUI libraries.
    This is important for pip users who only want the CLI.
    """
    before = set(sys.modules.keys())
    try:
        import pulsevault.cli  # should be safe even without [gui]
    except Exception as e:
        print(f"Failed to import pulsevault.cli: {e}")
        return False

    after = set(sys.modules.keys())
    gui_related = [m for m in after - before
                   if any(x in m.lower() for x in ("gui", "customtkinter", "tkinter", "tkdnd"))]

    if gui_related:
        print("WARNING: GUI modules were imported when loading CLI:")
        for m in gui_related:
            print("  ", m)
        return False

    print("Good: pulsevault.cli imported without pulling GUI/tkinter modules.")
    return True

def main():
    parser = argparse.ArgumentParser(description="Pulse-Vault build verification helper")
    parser.add_argument("--check-version", action="store_true", help="Check that the package reports a version")
    parser.add_argument("--cli-smoke", action="store_true", help="Run a quick non-interactive CLI test")
    parser.add_argument("--gui-import-isolated", action="store_true",
                        help="Verify that CLI does not import GUI code (run before installing [gui])")
    parser.add_argument("--all", action="store_true", help="Run all available checks")

    args = parser.parse_args()

    if not any([args.check_version, args.cli_smoke, args.gui_import_isolated, args.all]):
        parser.print_help()
        return 1

    results = []

    if args.all or args.check_version:
        results.append(("version", check_version()))

    if args.all or args.cli_smoke:
        results.append(("cli-smoke", cli_smoke()))

    if args.all or args.gui_import_isolated:
        results.append(("gui-isolated", gui_import_isolated()))

    print("\n=== Summary ===")
    all_ok = True
    for name, ok in results:
        status = "PASS" if ok else "FAIL"
        print(f"{name:20} {status}")
        if not ok:
            all_ok = False

    return 0 if all_ok else 1

if __name__ == "__main__":
    sys.exit(main())
