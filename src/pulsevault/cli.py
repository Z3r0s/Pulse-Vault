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
from pulsevault.core.vault import EncryptedVault, VaultError, human_size, password_policy_error, is_reasonable_password
from pulsevault.core.crypto import USER_SCRYPT_PROFILES, active_scrypt_profile



def _prompt_password(prompt: str = "Password: ", confirm: bool = False) -> str:
    """Secure password prompt using getpass."""
    while True:
        try:
            pw = getpass.getpass(prompt)
        except (EOFError, KeyboardInterrupt):
            print("\nPassword entry cancelled.")
            raise
        if not pw:
            print("Password cannot be empty.")
            continue
        if confirm:
            try:
                pw2 = getpass.getpass("Confirm password: ")
            except (EOFError, KeyboardInterrupt):
                print("\nPassword entry cancelled.")
                raise
            if pw != pw2:
                print("Passwords do not match. Try again.")
                continue
        return pw


def _prompt_profile(default: str = "standard", assume_yes: bool = False) -> str:
    """Prompt for scrypt profile, defaulting to standard. Auto-defaults with --yes for non-interactive."""
    if assume_yes:
        print(f"Using default profile '{default}' (non-interactive).")
        return default
    print("\nScrypt KDF profile (affects brute-force resistance and unlock speed):")
    print("  standard : balanced (~32 MiB RAM, good default)")
    print("  hardened : maximum resistance (~1 GiB RAM, slower)")
    print(f"  (test 'fast' profile is for development only; current runtime default: {active_scrypt_profile()})")
    try:
        choice = input(f"Profile [{default}]: ").strip().lower() or default
    except (EOFError, KeyboardInterrupt):
        print("\nUsing default profile 'standard'.")
        return "standard"
    if choice not in ("standard", "hardened"):
        print("Unrecognized choice, using standard.")
        choice = "standard"
    return choice


def _confirm_action(message: str, default: str = "n", assume_yes: bool = False) -> bool:
    """Consistent 'are you sure?' prompt beyond basic y/N.

    - Clearer safety wording for destructive ops (delete, pw change, overwrite, folder import).
    - Accepts 'y'/'yes' (case-insensitive); defaults to No for safety.
    - With --yes / assume_yes: auto-confirms and prints note (for scripting/headless/packaging).
    - Handles EOF/pipe for non-interactive robustness.
    """
    if assume_yes:
        print(f"{message}  [auto-confirmed via --yes]")
        return True
    suffix = " [y/N]" if default.lower() == "n" else " [Y/n]"
    try:
        resp = input(f"{message}{suffix}: ").strip().lower()
    except (EOFError, KeyboardInterrupt):
        print("\nCancelled (no confirmation).")
        return False
    if not resp:
        resp = default.lower()
    return resp in ("y", "yes")


def _cli_progress(label: str = "Working"):
    """Simple progress reporter for long-running CLI ops (add, extract, verify, folder import).
    Prints percentage updates for better UX on big files/folders/verify.
    """
    last_pct = [-1]
    def cb(done: int, total: int):
        if total <= 0:
            return
        pct = int(done * 100 / total)
        if pct != last_pct[0]:
            end = "\n" if pct >= 100 else ""
            print(f"\r{label}: {pct}% ({done}/{total})", end=end, flush=True)
            last_pct[0] = pct
    return cb


def _print_vault_info(vault: EncryptedVault, path: Path):
    try:
        meta = vault.stats()
        print(f"\nVault: {path}")
        print(f"  Files: {meta.get('file_count', 0)}")
        print(f"  Total size (plain): {human_size(meta.get('total_plain_size', 0))}")
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
    assume = getattr(args, "yes", False)
    profile = args.profile or _prompt_profile(assume_yes=assume)
    if profile not in USER_SCRYPT_PROFILES:
        profile = "standard"

    pw = _prompt_password("Enter vault password: ", confirm=True)
    err = password_policy_error(pw)
    if err:
        print(f"Warning: {err}")
        if not _confirm_action("Continue with weak or common password anyway?", default="n", assume_yes=assume):
            return 1

    carrier = Path(args.carrier).resolve() if args.carrier else None
    if carrier and not carrier.exists():
        print(f"Error: Carrier file not found: {carrier}")
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
        print(f"Error: Create failed: {e}")
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
        print(f"Error: Unlock failed: {e}")
        return 1

    _print_vault_info(vault, vault_path)
    assume = getattr(args, "yes", False)

    while True:
        print("\n--- Vault Menu ---")
        print("1. List files")
        print("2. Add file or folder")
        print("3. Extract file")
        print("4. Verify integrity")
        print("5. Change password (re-encrypts vault)")
        print("6. Delete file (permanent)")
        print("7. Stats / info")
        print("0. Lock and exit")
        try:
            choice = input("Choice [0-7]: ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nExiting menu.")
            try:
                vault.lock()
            except Exception:
                pass
            break

        if choice == "1":
            files = vault.list_files()
            if not files:
                print("(empty vault)")
            for f in files:
                m = vault.get_file_meta(f)
                print(f"  {f}  ({human_size(m.get('size', 0))})  sha256={m.get('sha256', '')[:16]}...")
        elif choice == "2":
            raw = input("Path to file or folder to add: ").strip()
            if not raw:
                continue
            p = Path(raw).expanduser().resolve()
            if not p.exists():
                print("Error: Path not found. Check spelling and use absolute path if needed.")
                continue
            try:
                if p.is_file():
                    fname = p.name
                    try:
                        vault.add_file(p)
                        print(f"Added file: {fname}")
                    except VaultError as ve:
                        msg = str(ve)
                        if "already exists" in msg.lower():
                            if _confirm_action(f"'{fname}' already exists in vault. Overwrite it?", default="n", assume_yes=assume):
                                vault.add_file(p, overwrite=True, progress_cb=_cli_progress(f"Encrypting {fname}"))
                                print(f"\nOverwrote: {fname}")
                            else:
                                print("Add cancelled.")
                        else:
                            raise
                elif p.is_dir():
                    # Precompute target archive name for overwrite check
                    zname = p.name.rstrip("/").rstrip("\\") + ".zip"
                    existing = zname in vault.list_files()
                    folder_msg = f"Import folder '{p.name}' by creating encrypted '{zname}' inside vault?"
                    if existing:
                        folder_msg = f"Folder archive '{zname}' already exists. Overwrite by re-importing folder '{p.name}'?"
                    if not _confirm_action(folder_msg + " (can take time and uses temp space for large folders)", default="n", assume_yes=assume):
                        print("Folder import cancelled.")
                        continue
                    prog = _cli_progress(f"Zipping+encrypting folder '{p.name}'")
                    vault.add_folder_as_zip(p, overwrite=existing or assume, progress_cb=prog)
                    print(f"\nAdded folder as archive: {zname}")
                else:
                    print("Error: Unsupported path type (only files and folders).")
            except Exception as e:
                print(f"Error: Add failed: {e}")
        elif choice == "3":
            name = input("Filename to extract (exact, from list): ").strip()
            if not name:
                continue
            outdir = input("Output directory [current dir]: ").strip() or "."
            out_path = Path(outdir).expanduser().resolve()
            try:
                outp = vault.extract_file(name, out_path)
                print(f"Extracted to: {outp}")
            except VaultError as ve:
                msg = str(ve)
                if "already exists" in msg.lower():
                    if _confirm_action(f"'{name}' already exists in '{out_path}'. Overwrite?", default="n", assume_yes=assume):
                        outp = vault.extract_file(name, out_path, overwrite=True, progress_cb=_cli_progress(f"Extracting {name}"))
                        print(f"Extracted (overwritten) to: {outp}")
                    else:
                        print("Extract cancelled.")
                else:
                    print(f"Error: Extract failed: {ve}")
            except Exception as e:
                print(f"Error: Extract failed: {e}")
        elif choice == "4":
            try:
                print("Verifying vault contents (reads and checks hashes)...")
                prog = _cli_progress("Verifying")
                res = vault.verify_all(progress_cb=prog)
                fc = res.get("file_count", 0)
                bc = res.get("bytes_checked", 0)
                hc = res.get("hash_checked_count", 0)
                print(f"\nVerify complete: {fc} files, {human_size(bc)} checked, {hc} full hash-verified.")
            except Exception as e:
                print(f"Error: Verify failed: {e}")
        elif choice == "5":
            if not _confirm_action(
                "Change vault password? This re-encrypts ALL file data with the new key. Old password will no longer work.",
                default="n",
                assume_yes=assume
            ):
                print("Password change cancelled.")
                continue
            old = _prompt_password("Current password: ")
            new = _prompt_password("New password: ", confirm=True)
            try:
                print("Starting full re-encryption (may take a while)...")
                vault.change_password(old, new)
                print("Password changed successfully and vault fully re-encrypted.")
            except Exception as e:
                print(f"Error: Password change failed: {e}")
        elif choice == "6":
            name = input("Filename to delete (exact): ").strip()
            if not name:
                continue
            try:
                meta = vault.get_file_meta(name)
                size = human_size(meta.get("size", 0))
            except Exception:
                size = "?"
            if not _confirm_action(
                f"PERMANENTLY DELETE '{name}' ({size}) from the vault? Data is removed and cannot be recovered.",
                default="n",
                assume_yes=assume
            ):
                print("Delete cancelled.")
                continue
            try:
                vault.delete_file(name)
                print(f"Deleted: {name}")
            except Exception as e:
                print(f"Error: Delete failed: {e}")
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
        print("Error: Vault not found.")
        return 1
    pw = _prompt_password()
    try:
        v = EncryptedVault(vault_path)
        v.unlock(pw)
        print("Verifying integrity...")
        res = v.verify_all()
        fc = res.get("file_count", 0)
        bc = res.get("bytes_checked", 0)
        hc = res.get("hash_checked_count", 0)
        print(f"Verify OK: {fc} files, {human_size(bc)} bytes checked, {hc} full hash-verified.")
        return 0
    except Exception as e:
        print(f"Error: Verify failed: {e}")
        return 1


def cmd_list(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    pw = _prompt_password()
    try:
        v = EncryptedVault(vault_path)
        v.unlock(pw)
        files = v.list_files()
        if not files:
            print("(no files in vault)")
        else:
            print(f"Files in {vault_path.name}:")
            for f in files:
                print(f"  {f}")
        return 0
    except Exception as e:
        print(f"Error: List failed: {e}")
        return 1


def cmd_change_pw(args: argparse.Namespace) -> int:
    vault_path = Path(args.vault).resolve()
    old = _prompt_password("Old password: ")
    new = _prompt_password("New password: ", confirm=True)
    assume = getattr(args, "yes", False)
    if not _confirm_action(
        "Apply password change? This will fully re-encrypt every file with the new password (can be slow for large vaults). Old password will stop working.",
        default="n",
        assume_yes=assume
    ):
        print("Password change cancelled.")
        return 0
    try:
        v = EncryptedVault(vault_path)
        v.unlock(old)
        v.change_password(old, new)
        print("Password rotated successfully (full re-encrypt with new key).")
        return 0
    except Exception as e:
        print(f"Error: Password change failed: {e}")
        return 1


def run_guided_cli(argv: Optional[list] = None) -> int:
    """Main entry for guided CLI mode."""
    parser = argparse.ArgumentParser(
        prog="pulse-vault",
        description=f"{__app_name__} guided CLI (v{__version__}). Use for terminal, headless, scripting or packaging (pip/apt/snap).",
        epilog="Safety: destructive actions use explicit 'are you sure?' prompts. Use -y/--yes for auto-confirm in non-interactive runs. Standard profile is default.",
    )
    sub = parser.add_subparsers(dest="command", required=False)

    # create
    p_create = sub.add_parser("create", help="Create a new vault (defaults to standard KDF; confirms on weak pw)")
    p_create.add_argument("vault", help="Path for new .pulsevault file")
    p_create.add_argument("--profile", choices=["standard", "hardened"], help="KDF profile (standard is balanced default)")
    p_create.add_argument("--carrier", help="Optional carrier media file to append vault to")
    p_create.add_argument("--yes", action="store_true", help="Assume yes for confirmations (non-interactive use)")
    p_create.set_defaults(func=cmd_create)

    # open (interactive session)
    p_open = sub.add_parser("open", help="Open vault and enter guided menu (improved safety prompts + progress)")
    p_open.add_argument("vault", help="Path to .pulsevault")
    p_open.add_argument("-y", "--yes", action="store_true", help="Auto-confirm for overwrites/deletes/pw-change/folder-imports (non-interactive/headless safe use)")
    p_open.set_defaults(func=cmd_open)

    # quick ops
    p_list = sub.add_parser("list", help="List files (read-only; prompts for password)")
    p_list.add_argument("vault", help="Path to .pulsevault")
    p_list.set_defaults(func=cmd_list)

    p_verify = sub.add_parser("verify", help="Verify vault integrity (read-only)")
    p_verify.add_argument("vault", help="Path to .pulsevault")
    p_verify.set_defaults(func=cmd_verify)

    p_chpw = sub.add_parser("change-password", help="Rotate password (full re-encrypt of vault; destructive)")
    p_chpw.add_argument("vault", help="Path to .pulsevault")
    p_chpw.add_argument("-y", "--yes", action="store_true", help="Auto-confirm the re-encrypt warning (non-interactive use; use with care)")
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
        print("  pulse-vault --cli change-password my.vault -y   # non-interactive example")
        print("Safety defaults: standard KDF; confirmations required for delete/password change.")
        print("Add -y/--yes to dangerous commands for non-interactive / headless / packaging scripts.")
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
