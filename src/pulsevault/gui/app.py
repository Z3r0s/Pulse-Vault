import atexit
import datetime
import os
import shutil
import subprocess
import sys
import tempfile
import threading
from pathlib import Path
from tkinter import filedialog, messagebox, ttk
from tkinter.simpledialog import askstring
from typing import List, Optional
import tkinter as tk

import customtkinter as ctk

from pulsevault import __version__
from pulsevault.core.vault import EncryptedVault, VaultError, safe_filename, secure_unlink
from pulsevault.gui.dialogs import ask_password, ask_scrypt_profile
from pulsevault.gui.theme import resolve_appearance_mode, tree_fonts, tree_palette


APP_NAME = "Pulse-Vault"
APP_SUBTITLE = "DNSPulse hardened local vault"
COMMON_PASSWORDS = {
    "password",
    "password123",
    "123456789012",
    "qwerty123456",
    "letmein123456",
    "adminadminadmin",
    "pulsevault",
}


def human_size(size: int) -> str:
    units = ["B", "KB", "MB", "GB", "TB"]
    value = float(size)
    for unit in units:
        if value < 1024:
            return f"{value:.1f} {unit}"
        value /= 1024
    return f"{value:.1f} PB"


def password_policy_error(password: str) -> Optional[str]:
    if len(password) < 14:
        return "Use at least 14 characters, or generate a secure key."

    lowered = password.lower()
    if lowered in COMMON_PASSWORDS or any(piece in lowered for piece in ("password", "qwerty", "letmein", "123456")):
        return "Avoid common words, keyboard patterns, and obvious number runs."

    if len(set(password)) < 5:
        return "Use a less repetitive password."

    categories = sum(
        (
            any(ch.islower() for ch in password),
            any(ch.isupper() for ch in password),
            any(ch.isdigit() for ch in password),
            any(not ch.isalnum() for ch in password),
        )
    )
    if categories < 3 and len(password) < 20:
        return "Use more variety, or make it at least 20 characters."

    return None


def is_reasonable_password(password: str) -> bool:
    return password_policy_error(password) is None


class VaultGUI(ctk.CTk):
    def __init__(self):
        super().__init__()

        self.title(f"{APP_NAME} v{__version__} - {APP_SUBTITLE}")
        self.geometry("1120x700")
        self.minsize(940, 620)

        self.grid_columnconfigure(1, weight=1)
        self.grid_rowconfigure(0, weight=1)

        self.vault: Optional[EncryptedVault] = None
        self.filtered_files: List[str] = []
        self._search_after_id = None
        self._icon_image = None
        self._status_restore = None
        self.secure_temp_dir = Path(tempfile.mkdtemp(prefix=".pulse_secure_"))
        try:
            self.secure_temp_dir.chmod(0o700)
        except Exception:
            pass
        atexit.register(self.cleanup_temp_dir)

        self.build_sidebar()
        self.build_main_view()
        self.setup_window_icon()
        self.apply_tree_theme()
        self.setup_drag_drop()
        self.update_empty_state()
        self.bind("<<RefreshList>>", lambda _: self.refresh_list())
        self.bind("<<ClearProgress>>", lambda _: self.hide_progress())

    def cleanup_temp_dir(self):
        try:
            if self.secure_temp_dir.exists():
                for path in sorted(self.secure_temp_dir.rglob("*"), reverse=True):
                    if path.is_symlink():
                        path.unlink(missing_ok=True)
                    elif path.is_file():
                        secure_unlink(path)
                    elif path.is_dir():
                        path.rmdir()
                shutil.rmtree(self.secure_temp_dir, ignore_errors=True)
        except Exception:
            pass

    def build_sidebar(self):
        self.sidebar_frame = ctk.CTkFrame(self, width=224, corner_radius=0)
        self.sidebar_frame.grid(row=0, column=0, rowspan=2, sticky="nsew")
        self.sidebar_frame.grid_rowconfigure(9, weight=1)

        self.logo_label = ctk.CTkLabel(
            self.sidebar_frame,
            text="Pulse-Vault",
            font=ctk.CTkFont(size=24, weight="bold"),
            text_color="#10b981",
        )
        self.logo_label.grid(row=0, column=0, padx=20, pady=(24, 0), sticky="w")

        self.version_badge = ctk.CTkLabel(
            self.sidebar_frame,
            text=f"v{__version__}\n{APP_SUBTITLE}",
            justify="left",
            font=ctk.CTkFont(size=11),
            text_color="#94a3b8",
        )
        self.version_badge.grid(row=1, column=0, padx=20, pady=(2, 18), sticky="w")

        ctk.CTkFrame(self.sidebar_frame, height=1, fg_color="#263241").grid(
            row=2, column=0, sticky="ew", padx=16, pady=(0, 14)
        )

        self.btn_new = ctk.CTkButton(
            self.sidebar_frame,
            text="+ New Vault",
            command=self.create_vault,
            fg_color="#10b981",
            hover_color="#059669",
            height=40,
            font=ctk.CTkFont(size=13, weight="bold"),
        )
        self.btn_new.grid(row=3, column=0, padx=20, pady=6, sticky="ew")

        self.btn_open = ctk.CTkButton(
            self.sidebar_frame,
            text="Open Vault",
            command=self.open_vault,
            fg_color="#2563eb",
            hover_color="#1d4ed8",
            height=40,
            font=ctk.CTkFont(size=13),
        )
        self.btn_open.grid(row=4, column=0, padx=20, pady=6, sticky="ew")

        self.btn_lock = ctk.CTkButton(
            self.sidebar_frame,
            text="Lock Vault",
            command=self.lock_vault,
            state="disabled",
            height=38,
            font=ctk.CTkFont(size=13),
        )
        self.btn_lock.grid(row=5, column=0, padx=20, pady=6, sticky="ew")

        self.btn_change_pw = ctk.CTkButton(
            self.sidebar_frame,
            text="Rotate Password",
            command=self.change_password,
            state="disabled",
            fg_color="transparent",
            border_width=1,
            height=36,
            font=ctk.CTkFont(size=12),
        )
        self.btn_change_pw.grid(row=6, column=0, padx=20, pady=6, sticky="ew")

        self.btn_verify = ctk.CTkButton(
            self.sidebar_frame,
            text="Verify Vault",
            command=self.verify_vault,
            state="disabled",
            fg_color="transparent",
            border_width=1,
            height=36,
            font=ctk.CTkFont(size=12),
        )
        self.btn_verify.grid(row=7, column=0, padx=20, pady=6, sticky="ew")

        self.btn_about = ctk.CTkButton(
            self.sidebar_frame,
            text="Security Notes",
            command=self.show_about,
            fg_color="transparent",
            border_width=1,
            text_color="#10b981",
            hover_color=("gray90", "gray20"),
            height=36,
            font=ctk.CTkFont(size=12),
        )
        self.btn_about.grid(row=8, column=0, padx=20, pady=(6, 18), sticky="ew")

        self.appearance_mode_label = ctk.CTkLabel(
            self.sidebar_frame,
            text="Appearance",
            anchor="w",
            font=ctk.CTkFont(size=11, weight="bold"),
            text_color="#94a3b8",
        )
        self.appearance_mode_label.grid(row=10, column=0, padx=20, pady=(8, 0), sticky="w")
        self.appearance_mode_optionemenu = ctk.CTkOptionMenu(
            self.sidebar_frame,
            values=["System", "Dark", "Light"],
            command=self.change_appearance_mode_event,
            height=32,
        )
        self.appearance_mode_optionemenu.grid(row=11, column=0, padx=20, pady=(6, 20), sticky="ew")

    def build_main_view(self):
        self.main_frame = ctk.CTkFrame(self, corner_radius=0, fg_color="transparent")
        self.main_frame.grid(row=0, column=1, sticky="nsew", padx=22, pady=22)
        self.main_frame.grid_columnconfigure(0, weight=1)
        self.main_frame.grid_rowconfigure(3, weight=1)

        self.top_bar = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.top_bar.grid(row=0, column=0, sticky="ew", pady=(0, 12))
        self.top_bar.grid_columnconfigure(0, weight=1)

        self.status_label = ctk.CTkLabel(
            self.top_bar,
            text="No vault loaded.",
            font=ctk.CTkFont(size=16, weight="bold"),
        )
        self.status_label.grid(row=0, column=0, sticky="w")

        self.stats_label = ctk.CTkLabel(
            self.top_bar,
            text="Files: 0 | Vault size: 0 B",
            text_color="#94a3b8",
        )
        self.stats_label.grid(row=1, column=0, sticky="w")

        self.security_label = ctk.CTkLabel(
            self.top_bar,
            text="Offline | Scrypt KDF | ChaCha20-Poly1305 + AES-GCM",
            font=ctk.CTkFont(size=11, weight="bold"),
            text_color="#10b981",
        )
        self.security_label.grid(row=0, column=1, rowspan=2, padx=(18, 0), sticky="e")

        self.warning_label = ctk.CTkLabel(
            self.main_frame,
            text="Secure Open uses a temporary plaintext copy. Extracted files and external viewers are outside vault protection.",
            anchor="w",
            text_color="#f59e0b",
            font=ctk.CTkFont(size=11),
        )
        self.warning_label.grid(row=1, column=0, sticky="ew", pady=(0, 10))

        self.search_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.search_frame.grid(row=2, column=0, sticky="ew", pady=(0, 10))
        self.search_frame.grid_columnconfigure(0, weight=1)

        self.search_entry = ctk.CTkEntry(self.search_frame, placeholder_text="Search encrypted file index...")
        self.search_entry.grid(row=0, column=0, sticky="ew", padx=(0, 10))
        self.search_entry.bind("<KeyRelease>", self.schedule_refresh_list)

        self.progress_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.progress_bar = ctk.CTkProgressBar(self.progress_frame)
        self.progress_bar.set(0)
        self.progress_label = ctk.CTkLabel(
            self.progress_frame,
            text="",
            font=ctk.CTkFont(size=11),
            text_color="#94a3b8",
        )

        self.tree_frame = ctk.CTkFrame(self.main_frame, corner_radius=6)
        self.tree_frame.grid(row=3, column=0, sticky="nsew", pady=(0, 10))
        self.tree_frame.grid_columnconfigure(0, weight=1)
        self.tree_frame.grid_rowconfigure(0, weight=1)

        self.empty_panel = ctk.CTkFrame(self.tree_frame, fg_color="transparent")
        self.empty_panel.grid(row=0, column=0, sticky="nsew")
        self.empty_panel.grid_columnconfigure(0, weight=1)
        self.empty_panel.grid_rowconfigure(0, weight=1)
        ctk.CTkLabel(
            self.empty_panel,
            text="No vault loaded",
            font=ctk.CTkFont(size=18, weight="bold"),
        ).grid(row=0, column=0, pady=(0, 6))
        ctk.CTkLabel(
            self.empty_panel,
            text="Create or open a vault from the sidebar.\nDrag files here once a vault is unlocked.",
            font=ctk.CTkFont(size=13),
            text_color="#94a3b8",
            justify="center",
        ).grid(row=1, column=0)

        self.tree_style = ttk.Style()
        self.tree_style.theme_use("default")

        columns = ("name", "size", "type", "added", "hash")
        self.tree = ttk.Treeview(
            self.tree_frame,
            columns=columns,
            show="headings",
            selectmode="extended",
            style="Pulse.Treeview",
        )
        self.tree.heading("name", text="Name", anchor="w")
        self.tree.heading("size", text="Size", anchor="e")
        self.tree.heading("type", text="Type", anchor="center")
        self.tree.heading("added", text="Added", anchor="center")
        self.tree.heading("hash", text="SHA-256", anchor="w")

        self.tree.column("name", width=360, anchor="w", minwidth=180)
        self.tree.column("size", width=90, anchor="e", minwidth=70)
        self.tree.column("type", width=95, anchor="center", minwidth=80)
        self.tree.column("added", width=145, anchor="center", minwidth=110)
        self.tree.column("hash", width=220, anchor="w", minwidth=140)

        yscroll = ttk.Scrollbar(self.tree_frame, orient="vertical", command=self.tree.yview)
        self.tree.configure(yscrollcommand=yscroll.set)
        self.tree.grid(row=0, column=0, sticky="nsew")
        yscroll.grid(row=0, column=1, sticky="ns")

        self.tree.bind("<Double-1>", lambda _: self.extract_selected())
        self.tree.bind("<Button-3>", self.show_context_menu)
        self.tree.bind("<<TreeviewSelect>>", lambda _: self.update_selection_label())

        self.context_menu = tk.Menu(self, tearoff=0)
        self.context_menu.add_command(label="Extract...", command=self.extract_selected)
        self.context_menu.add_command(label="Secure Open", command=self.secure_view)
        self.context_menu.add_separator()
        self.context_menu.add_command(label="Rename", command=self.rename_selected)
        self.context_menu.add_separator()
        self.context_menu.add_command(label="Delete", command=self.delete_selected)

        self.action_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.action_frame.grid(row=4, column=0, sticky="ew", pady=(6, 0))

        self.btn_add_file = ctk.CTkButton(self.action_frame, text="+ Add File", command=self.add_file, state="disabled", height=36)
        self.btn_add_file.pack(side="left", padx=(0, 8))
        self.btn_add_folder = ctk.CTkButton(self.action_frame, text="+ Add Folder", command=self.add_folder, state="disabled", height=36)
        self.btn_add_folder.pack(side="left", padx=(0, 8))
        self.btn_extract = ctk.CTkButton(self.action_frame, text="Extract", command=self.extract_selected, state="disabled", height=36)
        self.btn_extract.pack(side="left", padx=(0, 8))
        self.btn_view = ctk.CTkButton(
            self.action_frame,
            text="Secure Open",
            command=self.secure_view,
            state="disabled",
            fg_color="#f59e0b",
            hover_color="#d97706",
            height=36,
        )
        self.btn_view.pack(side="left", padx=(0, 8))

        self.selection_label = ctk.CTkLabel(self.action_frame, text="No selection", text_color="#94a3b8")
        self.selection_label.pack(side="left", padx=(10, 0))

        self.btn_delete = ctk.CTkButton(
            self.action_frame,
            text="Delete",
            command=self.delete_selected,
            state="disabled",
            fg_color="#dc2626",
            hover_color="#b91c1c",
            height=36,
        )
        self.btn_delete.pack(side="right")
        self.btn_rename = ctk.CTkButton(
            self.action_frame,
            text="Rename",
            command=self.rename_selected,
            state="disabled",
            fg_color="transparent",
            border_width=1,
            height=36,
        )
        self.btn_rename.pack(side="right", padx=(0, 8))

    def setup_drag_drop(self):
        try:
            from tkinterdnd2 import DND_FILES, TkinterDnD

            TkinterDnD._require(self)
        except Exception:
            return

        for widget in (self.tree, self.tree_frame):
            try:
                widget.drop_target_register(DND_FILES)
                widget.dnd_bind("<<Drop>>", self.on_drop_files)
            except Exception:
                pass

    def on_drop_files(self, event):
        if not self.vault or not self.vault.is_unlocked:
            messagebox.showinfo("No vault", "Open a vault before adding files.")
            return

        try:
            raw_paths = self.tk.splitlist(event.data)
        except tk.TclError:
            return

        paths = [Path(p) for p in raw_paths if Path(p).is_file()]
        if not paths:
            return

        def task():
            for path in paths:
                if path.name not in self.vault.data.get("files", {}):
                    self.vault.add_file(path, overwrite=True)

        self._run_in_thread(task)

    def setup_window_icon(self):
        icon_path = Path(__file__).resolve().parent.parent / "assets" / "pulse-vault.png"
        if not icon_path.exists():
            return
        try:
            self._icon_image = tk.PhotoImage(file=str(icon_path))
            self.iconphoto(True, self._icon_image)
        except Exception:
            pass

    def apply_tree_theme(self):
        mode = resolve_appearance_mode(ctk.get_appearance_mode())
        palette = tree_palette(mode)
        body_font, heading_font = tree_fonts(self)

        self.tree_style.configure(
            "Pulse.Treeview",
            background=palette["bg"],
            foreground=palette["fg"],
            rowheight=34,
            fieldbackground=palette["field"],
            borderwidth=0,
            font=body_font,
        )
        self.tree_style.map("Pulse.Treeview", background=[("selected", palette["select"])])
        self.tree_style.configure(
            "Pulse.Treeview.Heading",
            background=palette["heading_bg"],
            foreground=palette["heading_fg"],
            relief="flat",
            font=heading_font,
        )
        self.tree.tag_configure("odd", background=palette["odd"])
        self.tree.tag_configure("even", background=palette["even"])
        self.context_menu.configure(
            bg=palette["menu_bg"],
            fg=palette["menu_fg"],
            activebackground=palette["select"],
            activeforeground=palette["menu_fg"],
            font=body_font,
        )

    def update_empty_state(self):
        unlocked = bool(self.vault and self.vault.is_unlocked)
        if unlocked:
            self.empty_panel.grid_remove()
            self.tree.grid()
        else:
            self.tree.grid_remove()
            self.empty_panel.grid()

    def schedule_refresh_list(self, event=None):
        if self._search_after_id is not None:
            self.after_cancel(self._search_after_id)
        self._search_after_id = self.after(150, self._refresh_list_now)

    def change_appearance_mode_event(self, new_appearance_mode: str):
        ctk.set_appearance_mode(new_appearance_mode)
        self.apply_tree_theme()

    def set_status(self, message: str):
        self.status_label.configure(text=message)

    def update_selection_label(self):
        count = len(self.tree.selection())
        self.selection_label.configure(text=f"{count} selected" if count else "No selection")

    def update_stats(self):
        if not self.vault or not self.vault.is_unlocked:
            self.stats_label.configure(text="No vault loaded.")
            return

        try:
            stats = self.vault.stats()
            v = self.vault.version
            if v >= 5:
                fmt = "V5 | LZMA + Cascade | Scrypt"
            elif v == 4:
                fmt = "V4 | Cascade | Scrypt"
            elif v == 3:
                fmt = "V3 | Cascade | Scrypt"
            elif v == 2:
                fmt = "V2 | AES-GCM | PBKDF2"
            else:
                fmt = "V1 | Legacy"

            self.stats_label.configure(
                text=f"Files: {stats['file_count']}  |  "
                f"Vault size: {human_size(stats['vault_disk_size'])}  |  "
                f"Format: {fmt}"
            )
        except Exception:
            self.stats_label.configure(text="Stats unavailable.")

    def update_button_states(self, unlocked: bool):
        state = "normal" if unlocked else "disabled"
        for button in (
            self.btn_lock,
            self.btn_change_pw,
            self.btn_verify,
            self.btn_add_file,
            self.btn_add_folder,
            self.btn_extract,
            self.btn_view,
            self.btn_delete,
            self.btn_rename,
        ):
            button.configure(state=state)

    def require_vault(self) -> bool:
        if not self.vault or not self.vault.is_unlocked:
            messagebox.showwarning("No vault", "Create or open a vault first.")
            return False
        return True

    def show_progress(self, message: str = "Working..."):
        self._status_restore = self.status_label.cget("text")
        self.set_status(message)
        self.progress_frame.grid(row=2, column=0, sticky="ew", pady=(6, 10))
        self.progress_frame.grid_columnconfigure(0, weight=1)
        self.progress_bar.grid(row=0, column=0, sticky="ew")
        self.progress_label.grid(row=1, column=0, sticky="w", pady=(4, 0))
        self.progress_bar.set(0)
        self.progress_label.configure(text="")
        self.search_frame.grid_forget()

    def hide_progress(self):
        self.progress_frame.grid_forget()
        self.search_frame.grid(row=2, column=0, sticky="ew", pady=(0, 10))
        if self._status_restore:
            self.set_status(self._status_restore)
            self._status_restore = None

    def _update_progress(self, current: int, total: int, label: str = ""):
        if total > 0:
            self.progress_bar.stop()
            self.progress_bar.configure(mode="determinate")
            self.progress_bar.set(min(current / total, 1.0))
        if label:
            self.progress_label.configure(text=label)

    def show_context_menu(self, event):
        row = self.tree.identify_row(event.y)
        if row:
            self.tree.selection_set(row)
            self.context_menu.tk_popup(event.x_root, event.y_root)

    def refresh_list(self):
        if self._search_after_id is not None:
            self.after_cancel(self._search_after_id)
            self._search_after_id = None
        self._refresh_list_now()

    def _refresh_list_now(self):
        self._search_after_id = None
        for item in self.tree.get_children():
            self.tree.delete(item)

        self.filtered_files = []
        self.update_empty_state()
        if not self.vault or not self.vault.is_unlocked:
            self.update_stats()
            self.update_selection_label()
            return

        query = self.search_entry.get().strip().lower()
        row_index = 0
        for filename in self.vault.list_files():
            if query and query not in filename.lower():
                continue

            try:
                meta = self.vault.get_file_meta(filename)
            except Exception:
                continue

            file_type = "Folder ZIP" if meta.get("type") == "folder_zip" else "File"
            size = int(meta.get("size", 0))
            added_ts = meta.get("added_at", 0)
            try:
                added_str = datetime.datetime.fromtimestamp(added_ts).strftime("%Y-%m-%d %H:%M")
            except Exception:
                added_str = "-"

            digest = meta.get("sha256", "")
            if digest == "skipped_large_file":
                digest = "large file"
            elif digest:
                digest = digest[:16] + "..."
            else:
                digest = "-"

            tag = "even" if row_index % 2 == 0 else "odd"
            self.tree.insert("", "end", values=(filename, human_size(size), file_type, added_str, digest), tags=(tag,))
            self.filtered_files.append(filename)
            row_index += 1

        self.update_stats()
        self.update_selection_label()

    def create_vault(self):
        path = filedialog.asksaveasfilename(
            title="Create encrypted vault",
            defaultextension=".pulsevault",
            filetypes=[("Pulse-Vault files", "*.pulsevault"), ("Legacy PulseVault files", "*.PulseVault"), ("All files", "*.*")],
        )
        if not path:
            return

        carrier_path = None
        if messagebox.askyesno(
            "Carrier file",
            "Append the vault to an image or video carrier file?\n\n"
            "This is casual disguise, not forensic protection.",
        ):
            carrier = filedialog.askopenfilename(
                title="Select carrier image/video",
                filetypes=[("Media files", "*.png *.mp4 *.jpg"), ("All files", "*.*")],
            )
            if carrier:
                carrier_path = Path(carrier)

        scrypt_profile = ask_scrypt_profile(self)
        if not scrypt_profile:
            return

        password = ask_password(self, "Create Vault Password", confirm=True, show_generate=True)
        if not password:
            return
        password_error = password_policy_error(password)
        if password_error:
            messagebox.showwarning("Weak password", password_error)
            return

        create_state = {
            "vault": None,
            "error": None,
            "path": Path(path),
        }

        def create_task():
            try:
                vault = EncryptedVault(create_state["path"])
                vault.create(password, carrier_path=carrier_path, scrypt_profile=scrypt_profile)
                create_state["vault"] = vault
            except Exception as exc:
                create_state["error"] = exc

        def create_complete():
            if create_state["error"]:
                self.vault = None
                self.refresh_list()
                messagebox.showerror("Error", str(create_state["error"]))
                return
            self.vault = create_state["vault"]
            self.set_status(f"Unlocked: {create_state['path'].name}")
            self.update_button_states(True)
            self.refresh_list()

        profile_label = "hardened" if scrypt_profile == "hardened" else "standard"
        self._run_in_thread(create_task, create_complete, status=f"Creating vault ({profile_label})...")

    def open_vault(self):
        path = filedialog.askopenfilename(
            title="Open encrypted vault",
            filetypes=[("Pulse-Vault files", "*.pulsevault"), ("Legacy PulseVault files", "*.PulseVault"), ("All files", "*.*")],
        )
        if path:
            self.auto_open_vault(path)

    def auto_open_vault(self, path: str):
        target_path = Path(path)
        legacy_suffix = target_path.suffix in {".vault", ".PulseVault"}
        new_path = target_path.with_suffix(".pulsevault") if legacy_suffix else target_path

        if legacy_suffix and not new_path.exists():
            if not messagebox.askyesno(
                "Rename vault file",
                f"This vault uses the legacy {target_path.suffix} extension.\n\n"
                f"It will be renamed to:\n{new_path.name}\n\nContinue?",
            ):
                return
        elif legacy_suffix and new_path.exists():
            messagebox.showerror(
                "Rename blocked",
                f"A file named {new_path.name} already exists in that folder.",
            )
            return

        password = ask_password(self, "Unlock Vault")
        if not password:
            return

        unlock_state = {"vault": None, "error": None, "target_path": target_path}

        def unlock_task():
            try:
                vault = EncryptedVault(unlock_state["target_path"])
                vault.unlock(password)
                unlock_state["vault"] = vault
            except Exception as exc:
                unlock_state["error"] = exc

        def unlock_complete():
            if unlock_state["error"]:
                self.vault = None
                self.refresh_list()
                messagebox.showerror("Unlock failed", str(unlock_state["error"]))
                return

            vault = unlock_state["vault"]
            current_path = unlock_state["target_path"]

            if legacy_suffix and not new_path.exists():
                current_path.rename(new_path)
                vault.vault_path = new_path
                current_path = new_path

            if vault.version < 5:
                if messagebox.askyesno(
                    "Upgrade vault format",
                    f"This vault uses format V{vault.version}.\n\n"
                    "Upgrade to the current format now? All file entries will be "
                    "re-encrypted. Large vaults can take a while.",
                ):
                    vault.migrate_to_current_format(password)

            self.vault = vault
            self.set_status(f"Unlocked: {current_path.name}")
            self.update_button_states(True)
            self.refresh_list()

        profile = EncryptedVault.peek_scrypt_profile(target_path) or "standard"
        status = "Deriving key (hardened)..." if profile == "hardened" else "Deriving key..."
        self._run_in_thread(unlock_task, unlock_complete, status=status)

    def lock_vault(self):
        if self.vault:
            self.vault.lock()
        self.vault = None
        self.search_entry.delete(0, "end")
        self.set_status("Vault locked.")
        self.update_button_states(False)
        self.refresh_list()

    def _run_in_thread(self, task_func, on_complete=None, status: str = "Working..."):
        self.update_button_states(False)
        self.show_progress(status)
        self.progress_bar.configure(mode="indeterminate")
        self.progress_bar.start()

        def wrapper():
            error = None
            try:
                task_func()
            except Exception as e:
                error = e
            self.after(0, self._thread_complete, error, on_complete)

        threading.Thread(target=wrapper, daemon=True).start()

    def _thread_complete(self, error, on_complete):
        self.progress_bar.stop()
        self.progress_bar.configure(mode="determinate")
        self.event_generate("<<ClearProgress>>")
        self.update_button_states(bool(self.vault and self.vault.is_unlocked))
        self.event_generate("<<RefreshList>>")

        if error:
            messagebox.showerror("Operation Failed", str(error))
        elif on_complete:
            on_complete()

    def add_file(self):
        if not self.require_vault():
            return

        paths = filedialog.askopenfilenames(title="Choose files to add")
        if not paths:
            return

        skipped = []

        pending = [Path(p) for p in paths]

        def task():
            total = len(pending)
            for index, path in enumerate(pending, start=1):
                if path.name in self.vault.data.get("files", {}):
                    skipped.append(path.name)
                    continue

                def progress_cb(done, file_total, i=index, n=total, name=path.name):
                    label = f"Adding {name} ({i}/{n})"
                    if file_total > 0:
                        self.after(0, lambda d=done, t=file_total, l=label: self._update_progress(d, t, l))
                    else:
                        self.after(0, lambda l=label: self.progress_label.configure(text=l))

                self.vault.add_file(path, overwrite=True, progress_cb=progress_cb)

            if skipped:
                self.after(
                    0,
                    lambda: messagebox.showwarning(
                        "Skipped Files",
                        f"{len(skipped)} file(s) already exist in the vault:\n"
                        + "\n".join(skipped[:5])
                        + ("..." if len(skipped) > 5 else ""),
                    ),
                )

        self._run_in_thread(task, status="Adding files...")

    def add_folder(self):
        if not self.require_vault():
            return

        path = filedialog.askdirectory(title="Choose folder to ZIP and add")
        if not path:
            return

        self._run_in_thread(lambda: self.vault.add_folder_as_zip(Path(path), overwrite=True))

    def extract_selected(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return

        output_dir = filedialog.askdirectory(title="Choose extraction folder")
        if not output_dir:
            return

        filenames = [self.tree.item(s, "values")[0] for s in selections]
        output_path = Path(output_dir)
        existing = [fname for fname in filenames if (output_path / safe_filename(fname)).exists()]
        overwrite = False
        if existing:
            overwrite = messagebox.askyesno(
                "Overwrite existing files?",
                f"{len(existing)} selected file(s) already exist in that folder. Overwrite them?",
            )
            if not overwrite:
                return

        def task():
            total = len(filenames)
            for index, fname in enumerate(filenames, start=1):
                def progress_cb(done, file_total, i=index, n=total, name=fname):
                    label = f"Extracting {name} ({i}/{n})"
                    if file_total > 0:
                        self.after(0, lambda d=done, t=file_total, l=label: self._update_progress(d, t, l))
                    else:
                        self.after(0, lambda l=label: self.progress_label.configure(text=l))

                self.vault.extract_file(fname, output_path, overwrite=overwrite, progress_cb=progress_cb)

        def done():
            messagebox.showinfo("Extracted", f"Extracted {len(filenames)} file(s) to:\n{output_dir}")

        self._run_in_thread(task, done, status="Extracting files...")

    def delete_selected(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return

        filenames = [self.tree.item(s, "values")[0] for s in selections]
        if not messagebox.askyesno("Delete", f"Delete {len(filenames)} file(s) from the vault?"):
            return

        self._run_in_thread(lambda: [self.vault.delete_file(fname) for fname in filenames])

    def rename_selected(self):
        if not self.require_vault():
            return

        selection = self.tree.selection()
        if not selection or len(selection) > 1:
            messagebox.showwarning("Selection", "Select exactly one file to rename.")
            return

        old_name = self.tree.item(selection[0], "values")[0]
        new_name = askstring("Rename", "New filename:", initialvalue=old_name)
        if not new_name or new_name == old_name:
            return

        self._run_in_thread(lambda: self.vault.rename_file(old_name, new_name))

    def change_password(self):
        if not self.require_vault():
            return

        old_pw = ask_password(self, "Current Password")
        if not old_pw:
            return

        new_pw = ask_password(self, "New Password", confirm=True, show_generate=True)
        if not new_pw:
            return
        password_error = password_policy_error(new_pw)
        if password_error:
            messagebox.showwarning("Weak password", password_error)
            return

        def done():
            messagebox.showinfo("Password changed", "Vault password changed and file entries re-encrypted.")

        self._run_in_thread(lambda: self.vault.change_password(old_pw, new_pw), done)

    def verify_vault(self):
        if not self.require_vault():
            return

        result_holder = {}

        def wrapped_task():
            def progress_cb(current, total):
                self.after(0, lambda: self._update_progress(current, total, f"Verifying {current}/{total}"))
            result_holder["result"] = self.vault.verify_all(progress_cb=progress_cb)

        def done():
            result = result_holder.get("result", {})
            messagebox.showinfo(
                "Vault verified",
                "Vault integrity check completed.\n\n"
                f"Files checked: {result.get('file_count', 0)}\n"
                f"Plaintext bytes verified in memory: {human_size(result.get('bytes_checked', 0))}\n"
                f"SHA-256 hashes checked: {result.get('hash_checked_count', 0)}",
            )

        self._run_in_thread(wrapped_task, done, status="Verifying vault...")

    def secure_view(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return

        if not messagebox.askyesno(
            "Secure Open",
            "Secure Open extracts plaintext files to a temporary app directory before launching them.\n\n"
            "External viewers may create their own caches or recent-file entries. Continue?",
        ):
            return

        filenames = [self.tree.item(s, "values")[0] for s in selections]

        def open_file(path: Path):
            try:
                if os.name == "nt":
                    os.startfile(path)
                elif sys.platform == "darwin":
                    subprocess.Popen(["open", str(path)])
                else:
                    subprocess.Popen(["xdg-open", str(path)])
            except Exception as e:
                messagebox.showerror("Open Failed", f"Could not open file:\n{e}")

        def task():
            paths_to_open = []
            for fname in filenames:
                output_path = self.vault.extract_file(fname, self.secure_temp_dir, overwrite=True)
                paths_to_open.append(output_path)
            for path in paths_to_open:
                self.after(0, lambda p=path: open_file(p))

        self._run_in_thread(task)

    def show_about(self):
        about_win = ctk.CTkToplevel(self)
        about_win.title("Pulse-Vault Security Notes")
        about_win.geometry("820x580")
        about_win.resizable(False, False)

        about_win.update_idletasks()
        if self.winfo_viewable():
            x = self.winfo_x() + (self.winfo_width() // 2) - (820 // 2)
            y = self.winfo_y() + (self.winfo_height() // 2) - (580 // 2)
            about_win.geometry(f"+{x}+{y}")

        about_win.transient(self)
        about_win.grab_set()
        about_win.grid_columnconfigure(0, weight=1)
        about_win.grid_rowconfigure(1, weight=1)

        header = ctk.CTkFrame(about_win, fg_color="#0f172a", corner_radius=0)
        header.grid(row=0, column=0, sticky="ew")
        header.grid_columnconfigure(0, weight=1)

        ctk.CTkLabel(
            header,
            text="Pulse-Vault",
            font=ctk.CTkFont(size=28, weight="bold"),
            text_color="#10b981",
        ).grid(row=0, column=0, padx=30, pady=(24, 4))
        ctk.CTkLabel(
            header,
            text=f"Version {__version__} | DNSPulse hardened local vault",
            font=ctk.CTkFont(size=12),
            text_color="#94a3b8",
        ).grid(row=1, column=0, padx=30, pady=(0, 20))

        about_text = (
            "Pulse-Vault is a local encrypted file vault for keeping sensitive files in a portable "
            "container. It is designed to avoid network services, keep large-file operations streamed, "
            "and make the vault format easy to move between machines.\n\n"
            "ARCHITECTURE\n"
            "V5 vault entries are compressed with LZMA/XZ, then encrypted through a streaming cascade. "
            "Each encrypted chunk is authenticated with associated data that binds it to the stream header "
            "and chunk position.\n\n"
            "KEY DERIVATION\n"
            "Master keys are derived using Scrypt, a memory-hard KDF that raises the cost of password "
            "guessing. Strong, unique passwords are still required.\n\n"
            "CARRIER FILES\n"
            "Carrier mode appends vault ZIP data after an image or video. This can disguise the file in "
            "casual workflows, but it is not forensic protection.\n\n"
            "SECURE OPEN\n"
            "Secure Open extracts files into a randomized temporary directory which is removed when the "
            "app exits normally. The opened file is plaintext while viewed, and external applications may "
            "create caches or recent-file entries.\n\n"
            "No telemetry. No networking. No cloud service dependency.\n\n"
            "Official site: https://dnspulse.org"
        )

        textbox = ctk.CTkTextbox(
            about_win,
            wrap="word",
            font=ctk.CTkFont(family="Segoe UI", size=13),
            fg_color="transparent",
            text_color="#d1d5db",
        )
        textbox.grid(row=1, column=0, sticky="nsew", padx=36, pady=(20, 10))
        textbox.insert("1.0", about_text)
        textbox.configure(state="disabled")

        ctk.CTkButton(
            about_win,
            text="Close",
            command=about_win.destroy,
            width=140,
            height=40,
            font=ctk.CTkFont(size=14, weight="bold"),
            fg_color="#10b981",
            hover_color="#059669",
        ).grid(row=2, column=0, pady=(0, 28))
        about_win.focus()
