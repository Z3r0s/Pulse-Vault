"""Guided CLI for Pulse-Vault.

Provides interactive terminal interface for common operations.
Useful for headless use, scripting, or when GUI not available/desired.
Makes the tool suitable for pip, apt, snap, etc. installs.
"""

import argparse
import getpass
import sys
from pathlib import Path
from typing import Optional

from pulsevault import __version__, __app_name__
from pulsevault.core.vault import EncryptedVault, VaultError
from pulsevault.core.crypto import USER_SCRYPT_PROFILES, active_scrypt_profile
from pulsevault.gui.app import is_reasonable_password, password_policy_error, human_size


def _prompt_password(prompt: str = "Password: ", confirm: bool = False) -> str:
    """Secure password prompt using getpass."""
    while True:
        pw = getpass.getpass(prompt)
        if not pw:
            print("Password cannot be empty.")
            continue
        if confirm:
            pw2 = getpass.getpass("Confirm password: ")
            if pw != pw2:
                print("Passwords do not match. Try again.")
                continue
        return pw


def _prompt_profile(default: str = "standard") -> str:
    print("\nScrypt KDF profile (affects brute-force resistance and unlock speed):")
    print("  standard : balanced (~32 MiB RAM, good default)")
    print("  hardened : maximum resistance (~1 GiB RAM, slower)")
    print(f"  (test 'fast' profile is for development only; current runtime default: {active_scrypt_profile()})")
    choice = input(f"Profile [{default}]: ").strip().lower() or default
    if choice not in ("standard", "hardened"):
        print("Using standard.")
        choice = "standard"
    return choice


def _print_vault_info(vault: EncryptedVault, path: Path):
    try:
        meta = vault.stats()
        print(f"\nVault: {path}")
        print(f"  Files: {meta.get('file_count', 0)}")
        print(f"  Total size (plain): {human_size(meta.get('total_size', 0))}")
        print(f"  Profile: {vault.scrypt_profile}")
        print(f"  Format: {vault._format_marker.decode('ascii', errors='ignore') if vault._format_marker else 'unknown'}")
    except Exception:
        print(f"Vault: {path} (unlocked)")


def cmd_create(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    if vault_path.exists():
        print(f"Error: {vault_path} already exists.")
        return 1

    print(f"Creating new Pulse-Vault at: {vault_path}")
    profile = args.profile or _prompt_profile()
    if profile not in USER_SCRYPT_PROFILES:
        profile = "standard"

    pw = _prompt_password("Enter vault password: ", confirm=True)
    err = password_policy_error(pw)
    if err:
        print(f"Warning: {err}")
        if input("Continue anyway? [y/N]: ").lower() != "y":
            return 1

    carrier = Path(args.carrier).resolve() if args.carrier else None
    if carrier and not carrier.exists():
        print(f"Carrier not found: {carrier}")
        return 1

    try:
        vault = EncryptedVault(vault_path)
        vault.create(pw, carrier_path=carrier, scrypt_profile=profile)
        print(f"\nVault created successfully.")
        print(f"  KDF profile: {profile}")
        if carrier:
            print(f"  Carrier mode: appended to {carrier}")
        print("Use 'pulse-vault --cli open <path>' or launch the GUI to add files.")
        return 0
    except Exception as e:
        print(f"Create failed: {e}")
        return 1


def cmd_open(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    if not vault_path.exists():
        print(f"Error: Vault not found: {vault_path}")
        return 1

    pw = _prompt_password("Enter password to unlock: ")
    try:
        vault = EncryptedVault(vault_path)
        vault.unlock(pw)
    except VaultError as e:
        print(f"Unlock failed: {e}")
        return 1

    _print_vault_info(vault, vault_path)

    while True:
        print("\n--- Vault Menu ---")
        print("1. List files")
        print("2. Add file(s)")
        print("3. Extract file")
        print("4. Verify integrity")
        print("5. Change password")
        print("6. Delete file")
        print("7. Stats / info")
        print("0. Lock and exit")
        choice = input("Choice: ").strip()

        if choice == "1":
            files = vault.list_files()
            if not files:
                print("(empty vault)")
            for f in files:
                m = vault.get_file_meta(f)
                print(f"  {f}  ({human_size(m.get('size', 0))})  sha256={m.get('sha256', '')[:16]}...")
        elif choice == "2":
            raw = input("Path to file or folder to add: ").strip()
            p = Path(raw).expanduser().resolve()
            if not p.exists():
                print("Path not found.")
                continue
            try:
                if p.is_file():
                    vault.add_file(p)
                    print(f"Added: {p.name}")
                elif p.is_dir():
                    count = vault.add_folder_as_zip(p)  # note: may be heavy
                    print(f"Added folder as archive: {count} items?")
                else:
                    print("Unsupported path type.")
            except Exception as e:
                print(f"Add failed: {e}")
        elif choice == "3":
            name = input("Filename to extract (exact): ").strip()
            outdir = input("Output directory [current]: ").strip() or "."
            try:
                outp = vault.extract_file(name, Path(outdir))
                print(f"Extracted to: {outp}")
            except Exception as e:
                print(f"Extract failed: {e}")
        elif choice == "4":
            try:
                res = vault.verify_all()
                print(f"Verify: {res.get('verified',0)}/{res.get('total',0)} OK. Errors: {len(res.get('errors',[]))}")
            except Exception as e:
                print(f"Verify error: {e}")
        elif choice == "5":
            old = _prompt_password("Current password: ")
            new = _prompt_password("New password: ", confirm=True)
            try:
                vault.change_password(old, new)
                print("Password changed and vault re-encrypted.")
            except Exception as e:
                print(f"Change failed: {e}")
        elif choice == "6":
            name = input("Filename to delete: ").strip()
            if input(f"Delete {name}? [y/N]: ").lower() == "y":
                try:
                    vault.delete_file(name)
                    print("Deleted.")
                except Exception as e:
                    print(f"Delete failed: {e}")
        elif choice == "7":
            _print_vault_info(vault, vault_path)
        elif choice in ("0", "q", "quit", "exit"):
            vault.lock()
            print("Locked.")
            break
        else:
            print("Invalid choice.")
    return 0


def cmd_verify(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    if not vault_path.exists():
        print("Vault not found.")
        return 1
    pw = _prompt_password()
    try:
        v = EncryptedVault(vault_path)
        v.unlock(pw)
        res = v.verify_all()
        print(f"Verified {res.get('verified',0)} / {res.get('total',0)} files.")
        if res.get("errors"):
            for err in res["errors"]:
                print("  ERROR:", err)
        return 0 if not res.get("errors") else 2
    except Exception as e:
        print(f"Failed: {e}")
        return 1


def cmd_list(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    pw = _prompt_password()
    try:
        v = EncryptedVault(vault_path)
        v.unlock(pw)
        files = v.list_files()
        if not files:
            print("(no files)")
        else:
            for f in files:
                print(f)
        return 0
    except Exception as e:
        print(f"List failed: {e}")
        return 1


def cmd_change_pw(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    old = _prompt_password("Old password: ")
    new = _prompt_password("New password: ", confirm=True)
    try:
        v = EncryptedVault(vault_path)
        v.unlock(old)
        v.change_password(old, new)
        print("Password rotated successfully (full re-encrypt).")
        return 0
    except Exception as e:
        print(f"Failed: {e}")
        return 1


def run_guided_cli(argv: Optional[list] = None) -> int:
    """Main entry for guided CLI mode."""
    parser = argparse.ArgumentParser(
        prog="pulse-vault",
        description=f"{__app_name__} guided CLI (v{__version__}). Use for terminal/headless or packaging-friendly access.",
    )
    sub = parser.add_subparsers(dest="command", required=False)

    # create
    p_create = sub.add_parser("create", help="Create a new vault (interactive prompts)")
    p_create.add_argument("vault", help="Path for new .pulsevault file")
    p_create.add_argument("--profile", choices=["standard", "hardened"], help="KDF profile")
    p_create.add_argument("--carrier", help="Optional carrier media file to append vault to")
    p_create.set_defaults(func=cmd_create)

    # open (interactive session)
    p_open = sub.add_parser("open", help="Open vault and enter guided menu")
    p_open.add_argument("vault", help="Path to .pulsevault")
    p_open.set_defaults(func=cmd_open)

    # quick ops
    p_list = sub.add_parser("list", help="List files (prompts for pw)")
    p_list.add_argument("vault")
    p_list.set_defaults(func=cmd_list)

    p_verify = sub.add_parser("verify", help="Verify vault integrity")
    p_verify.add_argument("vault")
    p_verify.set_defaults(func=cmd_verify)

    p_chpw = sub.add_parser("change-password", help="Rotate password")
    p_chpw.add_argument("vault")
    p_chpw.set_defaults(func=cmd_change_pw)

    # no subcommand -> show menu or help
    args = parser.parse_args(argv)

    if not args.command:
        # Guided top level
        print(f"{__app_name__} v{__version__} - Guided CLI")
        print("No subcommand given. Common usage:")
        print("  pulse-vault --cli create my.vault")
        print("  pulse-vault --cli open my.vault")
        print("  pulse-vault --cli list my.vault")
        print("  pulse-vault --cli verify my.vault")
        print("Run with -h for full help. Launch GUI with: pulse-vault [vault]")
        parser.print_help()
        return 0

    if hasattr(args, "func"):
        try:
            return args.func(args)
        except KeyboardInterrupt:
            print("\nCancelled.")
            return 130
    return 0


def main_cli_entry():
    """Console entry wrapper."""
    sys.exit(run_guided_cli())


if __name__ == "__main__":
    raise SystemExit(run_guided_cli())
