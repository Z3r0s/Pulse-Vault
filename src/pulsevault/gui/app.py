import atexit
import datetime
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import webbrowser
from pathlib import Path
from tkinter import filedialog, messagebox, ttk
from tkinter.simpledialog import askstring
from typing import List, Optional
import tkinter as tk

import customtkinter as ctk

from pulsevault import __version__
from pulsevault.core.vault import (
    EncryptedVault,
    VaultError,
    safe_filename,
    secure_unlink,
    human_size,
    password_policy_error,
    is_reasonable_password,
)
from pulsevault.gui.dialogs import ask_password, ask_scrypt_profile, GitHubReleasesDialog, show_github_releases_dialog
from pulsevault.gui.theme import (
    resolve_appearance_mode,
    tree_fonts,
    tree_palette,
    get_yaru_colors,
    adaptive_color,
    CORNER_RADIUS,
    CORNER_RADIUS_SMALL,
    BUTTON_HEIGHT_PRIMARY,
    BUTTON_HEIGHT,
    BUTTON_HEIGHT_COMPACT,
    get_ubuntu_font,
    get_adaptive_accent,
    get_adaptive_accent_hover,
    get_adaptive_gray,
    get_adaptive_warning,
    get_adaptive_success,
    get_yaru_button_colors,
    get_progress_color,
    get_search_border_color,
    YARU_ORANGE,
    YARU_RED,
    YARU_PAD,
    YARU_PAD_SMALL,
)


APP_NAME = "Pulse-Vault"
APP_SUBTITLE = "DNSPulse hardened local vault"
GITHUB_RELEASES_URL = "https://github.com/Z3r0s/Pulse-Vault/releases"
OFFICIAL_SITE = "https://dnspulse.org"


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
        self._prev_status = None
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
        self.set_status("Ready — create or open a vault (default profile: standard).")
        self.bind("<<RefreshList>>", lambda _: self.refresh_list())
        self.bind("<<ClearProgress>>", lambda _: self.hide_progress())

        # Keyboard shortcuts (enhancement for power users and GitHub binary users)
        self.bind("<Control-n>", lambda e: self.create_vault())
        self.bind("<Control-o>", lambda e: self.open_vault())
        self.bind("<Control-l>", lambda e: self.lock_vault())
        self.bind("<Control-f>", lambda e: self.search_entry.focus_set())
        self.bind("<F5>", lambda e: self.refresh_list())
        self.bind("<Delete>", lambda e: self.delete_selected() if self.vault and self.vault.is_unlocked else None)

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
        self.sidebar_frame.grid_rowconfigure(10, weight=1)

        self.logo_label = ctk.CTkLabel(
            self.sidebar_frame,
            text="Pulse-Vault",
            font=ctk.CTkFont(**get_ubuntu_font(24, "bold")),
            text_color=get_adaptive_accent(),  # Ubuntu 26.04 Yaru orange accent
        )
        self.logo_label.grid(row=0, column=0, padx=20, pady=(24, 0), sticky="w")

        self.version_badge = ctk.CTkLabel(
            self.sidebar_frame,
            text=f"v{__version__}\n{APP_SUBTITLE}",
            justify="left",
            font=ctk.CTkFont(**get_ubuntu_font(11)),
            text_color=get_adaptive_gray(),
            cursor="hand2",
        )
        self.version_badge.grid(row=1, column=0, padx=20, pady=(2, 18), sticky="w")
        self.version_badge.bind("<Button-1>", lambda e: self.open_github_releases())

        ctk.CTkFrame(self.sidebar_frame, height=1, fg_color=adaptive_color("#d0d0d0", "#363636")).grid(
            row=2, column=0, sticky="ew", padx=16, pady=(0, 14)
        )

        self.btn_new = ctk.CTkButton(
            self.sidebar_frame,
            text="+ New Vault",
            command=self.create_vault,
            fg_color=get_adaptive_accent(),
            hover_color=get_adaptive_accent_hover(),
            height=BUTTON_HEIGHT_PRIMARY,
            font=ctk.CTkFont(**get_ubuntu_font(13, "bold")),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_new.grid(row=3, column=0, padx=20, pady=6, sticky="ew")

        self.btn_open = ctk.CTkButton(
            self.sidebar_frame,
            text="Open Vault",
            command=self.open_vault,
            fg_color=adaptive_color("#2563eb", "#1d4ed8"),  # keep distinct blue for open; orange primary for new
            hover_color=adaptive_color("#1d4ed8", "#163a9e"),
            height=BUTTON_HEIGHT_PRIMARY,
            font=ctk.CTkFont(**get_ubuntu_font(13)),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_open.grid(row=4, column=0, padx=20, pady=6, sticky="ew")

        self.btn_lock = ctk.CTkButton(
            self.sidebar_frame,
            text="Lock Vault",
            command=self.lock_vault,
            state="disabled",
            height=BUTTON_HEIGHT,
            font=ctk.CTkFont(**get_ubuntu_font(13)),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_lock.grid(row=5, column=0, padx=20, pady=6, sticky="ew")

        self.btn_change_pw = ctk.CTkButton(
            self.sidebar_frame,
            text="Change Password",
            command=self.change_password,
            state="disabled",
            fg_color="transparent",
            border_width=1,
            height=BUTTON_HEIGHT,
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_change_pw.grid(row=6, column=0, padx=20, pady=6, sticky="ew")

        self.btn_verify = ctk.CTkButton(
            self.sidebar_frame,
            text="Verify Vault",
            command=self.verify_vault,
            state="disabled",
            fg_color="transparent",
            border_width=1,
            height=BUTTON_HEIGHT,
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_verify.grid(row=7, column=0, padx=20, pady=6, sticky="ew")

        self.btn_about = ctk.CTkButton(
            self.sidebar_frame,
            text="Security Notes",
            command=self.show_about,
            fg_color="transparent",
            border_width=1,
            text_color=get_adaptive_accent(),  # Yaru orange accent
            hover_color=adaptive_color("#e8e8e8", "#333333"),
            height=BUTTON_HEIGHT,
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_about.grid(row=8, column=0, padx=20, pady=6, sticky="ew")

        # NEW: Dedicated GitHub downloads button - always enabled, ties directly to GitHub Releases area
        self.btn_downloads = ctk.CTkButton(
            self.sidebar_frame,
            text="GitHub Releases",
            command=self.open_github_releases,
            fg_color="transparent",
            border_width=1,
            text_color=adaptive_color("#3b82f6", "#60a5fa"),  # keep blue distinction for external
            hover_color=adaptive_color("#e8e8e8", "#333333"),
            height=BUTTON_HEIGHT,
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_downloads.grid(row=9, column=0, padx=20, pady=(0, 12), sticky="ew")

        # Sidebar hover hints (non-destructive mostly)
        self._bind_hover_status(self.btn_new, "Create a new encrypted vault file (default: standard profile)")
        self._bind_hover_status(self.btn_open, "Open an existing .pulsevault file")
        self._bind_hover_status(self.btn_lock, "Lock current vault (clears in-memory keys)")
        self._bind_hover_status(self.btn_change_pw, "Rotate master password (re-encrypts all entries)")
        self._bind_hover_status(self.btn_verify, "Run full integrity verification of vault contents")

        self.appearance_mode_label = ctk.CTkLabel(
            self.sidebar_frame,
            text="Appearance",
            anchor="w",
            font=ctk.CTkFont(**get_ubuntu_font(11, "bold")),
            text_color=get_adaptive_gray(),
        )
        self.appearance_mode_label.grid(row=11, column=0, padx=20, pady=(8, 0), sticky="w")
        self.appearance_mode_optionmenu = ctk.CTkOptionMenu(
            self.sidebar_frame,
            values=["System", "Dark", "Light"],
            command=self.change_appearance_mode_event,
            height=BUTTON_HEIGHT_COMPACT,
            corner_radius=CORNER_RADIUS_SMALL,
            fg_color=get_adaptive_accent(),  # Yaru orange accent on mode switcher
            button_color=get_adaptive_accent_hover(),
            button_hover_color=get_adaptive_accent(),
        )
        self.appearance_mode_optionmenu.grid(row=12, column=0, padx=20, pady=(6, 20), sticky="ew")

    def build_main_view(self):
        self.main_frame = ctk.CTkFrame(self, corner_radius=0, fg_color="transparent")
        self.main_frame.grid(row=0, column=1, sticky="nsew", padx=22, pady=22)
        self.main_frame.grid_columnconfigure(0, weight=1)
        self.main_frame.grid_rowconfigure(3, weight=1)

        self.top_bar = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.top_bar.grid(row=0, column=0, sticky="ew", pady=(0, YARU_PAD))
        self.top_bar.grid_columnconfigure(0, weight=1)

        self.status_label = ctk.CTkLabel(
            self.top_bar,
            text="No vault loaded.",
            font=ctk.CTkFont(**get_ubuntu_font(16, "bold")),
        )
        self.status_label.grid(row=0, column=0, sticky="w")

        self.stats_label = ctk.CTkLabel(
            self.top_bar,
            text="Files: 0 | Vault size: 0 B",
            text_color=get_adaptive_gray(),
            font=ctk.CTkFont(**get_ubuntu_font(12)),
        )
        self.stats_label.grid(row=1, column=0, sticky="w")

        self.security_label = ctk.CTkLabel(
            self.top_bar,
            text="Offline | Scrypt KDF | ChaCha20-Poly1305 + AES-GCM",
            font=ctk.CTkFont(**get_ubuntu_font(11, "bold")),
            text_color=get_adaptive_success(),
        )
        self.security_label.grid(row=0, column=1, rowspan=2, padx=(18, 0), sticky="e")

        self.warning_label = ctk.CTkLabel(
            self.main_frame,
            text="Secure Open uses a temporary plaintext copy. Extracted files and external viewers are outside vault protection.",
            anchor="w",
            text_color=get_adaptive_warning(),
            font=ctk.CTkFont(**get_ubuntu_font(11)),
        )
        self.warning_label.grid(row=1, column=0, sticky="ew", pady=(0, YARU_PAD_SMALL))

        self.search_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.search_frame.grid(row=2, column=0, sticky="ew", pady=(0, YARU_PAD_SMALL))
        self.search_frame.grid_columnconfigure(0, weight=1)

        self.search_entry = ctk.CTkEntry(
            self.search_frame,
            placeholder_text="Search encrypted file index... (Ctrl+F to focus)",
            border_color=get_search_border_color(),
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            height=BUTTON_HEIGHT_COMPACT,
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.search_entry.grid(row=0, column=0, sticky="ew", padx=(0, 10))
        self.search_entry.bind("<KeyRelease>", self.schedule_refresh_list)

        self.progress_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.progress_bar = ctk.CTkProgressBar(self.progress_frame, progress_color=get_progress_color())
        self.progress_bar.set(0)
        self.progress_label = ctk.CTkLabel(
            self.progress_frame,
            text="",
            font=ctk.CTkFont(**get_ubuntu_font(11)),
            text_color=get_adaptive_gray(),
        )

        self.tree_frame = ctk.CTkFrame(self.main_frame, corner_radius=CORNER_RADIUS_SMALL)
        self.tree_frame.grid(row=3, column=0, sticky="nsew", pady=(0, YARU_PAD_SMALL))
        self.tree_frame.grid_columnconfigure(0, weight=1)
        self.tree_frame.grid_rowconfigure(0, weight=1)

        self.empty_panel = ctk.CTkFrame(self.tree_frame, fg_color=adaptive_color("#f8f8f8", "#2a2a2a"), corner_radius=CORNER_RADIUS_SMALL)
        self.empty_panel.grid(row=0, column=0, sticky="nsew")
        self.empty_panel.grid_columnconfigure(0, weight=1)
        self.empty_panel.grid_rowconfigure(0, weight=1)
        ctk.CTkLabel(
            self.empty_panel,
            text="No vault loaded",
            font=ctk.CTkFont(**get_ubuntu_font(18, "bold")),
            text_color=get_adaptive_accent(),
        ).grid(row=0, column=0, pady=(0, 6))
        ctk.CTkLabel(
            self.empty_panel,
            text="Create or open a vault from the sidebar.\n"
            "Default profile: 'standard' (recommended).\n"
            "Drag & drop files here or use the + Add buttons (once unlocked).\n"
            "Keyboard: Ctrl+N new, Ctrl+O open, Del delete, Ctrl+F search, F5 refresh.\n\n"
            "All data stays local. No cloud, no telemetry.",
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            text_color=get_adaptive_gray(),
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
        self.context_menu.add_command(label="Extract Selected...", command=self.extract_selected)
        self.context_menu.add_command(label="Secure Open", command=self.secure_view)
        self.context_menu.add_separator()
        self.context_menu.add_command(label="Rename", command=self.rename_selected)
        self.context_menu.add_separator()
        self.context_menu.add_command(label="Delete", command=self.delete_selected)

        self.action_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.action_frame.grid(row=4, column=0, sticky="ew", pady=(YARU_PAD_SMALL, 0))

        self.btn_add_file = ctk.CTkButton(self.action_frame, text="+ Add File", command=self.add_file, state="disabled", height=BUTTON_HEIGHT, corner_radius=CORNER_RADIUS_SMALL)
        self.btn_add_file.pack(side="left", padx=(0, 8))
        self.btn_add_folder = ctk.CTkButton(self.action_frame, text="+ Add Folder", command=self.add_folder, state="disabled", height=BUTTON_HEIGHT, corner_radius=CORNER_RADIUS_SMALL)
        self.btn_add_folder.pack(side="left", padx=(0, 8))
        self.btn_extract = ctk.CTkButton(self.action_frame, text="Extract Selected", command=self.extract_selected, state="disabled", height=BUTTON_HEIGHT, corner_radius=CORNER_RADIUS_SMALL)
        self.btn_extract.pack(side="left", padx=(0, 8))
        self.btn_view = ctk.CTkButton(
            self.action_frame,
            text="Secure Open",
            command=self.secure_view,
            state="disabled",
            fg_color=get_adaptive_warning(),
            hover_color=adaptive_color("#c2410f", "#e0702e"),
            height=BUTTON_HEIGHT,
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_view.pack(side="left", padx=(0, 8))

        self.selection_label = ctk.CTkLabel(
            self.action_frame,
            text="No selection",
            text_color=get_adaptive_gray(),
            font=ctk.CTkFont(**get_ubuntu_font(11)),
        )
        self.selection_label.pack(side="left", padx=(10, 0))

        danger_colors = get_yaru_button_colors("danger")
        self.btn_delete = ctk.CTkButton(
            self.action_frame,
            text="Delete",
            command=self.delete_selected,
            state="disabled",
            fg_color=danger_colors["fg_color"],
            hover_color=danger_colors["hover_color"],
            height=BUTTON_HEIGHT,
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_delete.pack(side="right")
        self.btn_rename = ctk.CTkButton(
            self.action_frame,
            text="Rename",
            command=self.rename_selected,
            state="disabled",
            fg_color="transparent",
            border_width=1,
            height=BUTTON_HEIGHT,
            corner_radius=CORNER_RADIUS_SMALL,
        )
        self.btn_rename.pack(side="right", padx=(0, 8))

        # Hover status hints for safety/UX (Yaru desktop feel)
        self._bind_hover_status(self.btn_add_file, "Add file(s) to vault (Ctrl+N for new vault)")
        self._bind_hover_status(self.btn_add_folder, "Add folder as compressed ZIP entry")
        self._bind_hover_status(self.btn_extract, "Extract selected to a folder (double-click also works)")
        self._bind_hover_status(self.btn_view, "Secure Open: extract temp + launch (external viewers leave traces)")
        self._bind_hover_status(self.btn_delete, "Permanently remove selected from vault (no undo)")
        self._bind_hover_status(self.btn_rename, "Rename a single selected entry inside the vault")

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

        existing = [p.name for p in paths if p.name in self.vault.data.get("files", {})]
        overwrite = False
        if existing:
            overwrite = messagebox.askyesno(
                "Overwrite on drop?",
                f"{len(existing)} dropped file(s) already exist in the vault.\n\n"
                "Overwrite them?",
            )
            if not overwrite:
                paths = [p for p in paths if p.name not in existing]

        if not paths:
            self.set_status("Drop cancelled (no new files).")
            return

        self.set_status(f"Dropping {len(paths)} file(s)...")

        def task():
            for path in paths:
                self.vault.add_file(path, overwrite=overwrite)

            self.after(0, lambda: self.set_status(f"Dropped and added {len(paths)} file(s)."))

        self._run_in_thread(task)

    def setup_window_icon(self):
        """Load window icon. Prefers .ico on Windows for best desktop/taskbar results."""
        assets_dir = Path(__file__).resolve().parent.parent / "assets"
        # Prefer platform-native .ico on win32
        icon_path = None
        if sys.platform.startswith("win"):
            ico = assets_dir / "pulse-vault.ico"
            if ico.exists():
                icon_path = ico
        if not icon_path:
            icon_path = assets_dir / "pulse-vault.png"
        if not icon_path.exists():
            return
        try:
            if icon_path.suffix.lower() == ".ico":
                # iconbitmap works well for .ico on Windows
                self.iconbitmap(str(icon_path))
            else:
                self._icon_image = tk.PhotoImage(file=str(icon_path))
                self.iconphoto(True, self._icon_image)
                # Also try iconbitmap on some platforms if png converted internally
                if sys.platform.startswith("win"):
                    try:
                        self.iconbitmap(str(icon_path))
                    except Exception:
                        pass
        except Exception:
            # Non-fatal; window will use default
            pass

    def apply_tree_theme(self):
        mode = resolve_appearance_mode(ctk.get_appearance_mode())
        palette = tree_palette(mode)
        body_font, heading_font = tree_fonts(self)

        # Treeview improvements: Yaru colors, consistent row height, orange select accent, Ubuntu fonts
        self.tree_style.configure(
            "Pulse.Treeview",
            background=palette["bg"],
            foreground=palette["fg"],
            rowheight=32,
            fieldbackground=palette["field"],
            borderwidth=0,
            font=body_font,
            bordercolor=palette.get("border", palette["bg"]),
        )
        self.tree_style.map(
            "Pulse.Treeview",
            background=[("selected", palette["select"])],
            foreground=[("selected", palette.get("select_fg", "#ffffff"))],
        )
        self.tree_style.configure(
            "Pulse.Treeview.Heading",
            background=palette["heading_bg"],
            foreground=palette["heading_fg"],
            relief="flat",
            font=heading_font,
            borderwidth=0,
        )
        self.tree.tag_configure("odd", background=palette["odd"])
        self.tree.tag_configure("even", background=palette["even"])
        self.context_menu.configure(
            bg=palette["menu_bg"],
            fg=palette["menu_fg"],
            activebackground=palette["select"],
            activeforeground=palette.get("select_fg", palette["menu_fg"]),
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

    def _bind_hover_status(self, widget, message: str):
        """Lightweight hover feedback for buttons (Ubuntu style hint via status)."""
        def on_enter(_):
            self._prev_status = self.status_label.cget("text")
            self.status_label.configure(text=message)
        def on_leave(_):
            if hasattr(self, "_prev_status") and self._prev_status:
                self.status_label.configure(text=self._prev_status)
            else:
                self.status_label.configure(text="Ready.")
        widget.bind("<Enter>", on_enter)
        widget.bind("<Leave>", on_leave)

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
        # Downloads button is always enabled (not vault-dependent)
        if hasattr(self, "btn_downloads"):
            self.btn_downloads.configure(state="normal")

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
        self.search_frame.grid(row=2, column=0, sticky="ew", pady=(0, YARU_PAD_SMALL))
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
            "Carrier file (optional)",
            "Append the vault data after an image or video file?\n\n"
            "This provides only casual disguise and is not strong obfuscation or forensic resistance.\n"
            "Continue?",
        ):
            carrier = filedialog.askopenfilename(
                title="Select carrier image/video",
                filetypes=[("Media files", "*.png *.mp4 *.jpg"), ("All files", "*.*")],
            )
            if carrier:
                carrier_path = Path(carrier)

        scrypt_profile = ask_scrypt_profile(self) or "standard"

        password = ask_password(self, "Create Vault Password", confirm=True, show_generate=True)
        if not password:
            return
        password_error = password_policy_error(password)
        if password_error:
            messagebox.showwarning("Weak password", password_error)
            if not messagebox.askyesno("Continue anyway?", "Use this password despite the warning?"):
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
                    f"This vault uses older format V{vault.version}.\n\n"
                    "Upgrade to current now? All entries will be re-encrypted under the new format.\n"
                    "Large vaults may take time. Recommended for best compatibility.",
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

        pending = [Path(p) for p in paths]
        existing = [p.name for p in pending if p.name in self.vault.data.get("files", {})]
        overwrite = False
        if existing:
            overwrite = messagebox.askyesno(
                "Overwrite on add?",
                f"{len(existing)} selected file(s) already exist in the vault.\n\n"
                "Overwrite the existing entries with these files?",
            )
            if not overwrite:
                pending = [p for p in pending if p.name not in existing]

        if not pending:
            return

        def task():
            total = len(pending)
            for index, path in enumerate(pending, start=1):
                def progress_cb(done, file_total, i=index, n=total, name=path.name):
                    label = f"Adding {name} ({i}/{n})"
                    if file_total > 0:
                        self.after(0, lambda d=done, t=file_total, l=label: self._update_progress(d, t, l))
                    else:
                        self.after(0, lambda l=label: self.progress_label.configure(text=l))

                self.vault.add_file(path, overwrite=overwrite, progress_cb=progress_cb)

            self.after(0, lambda: self.set_status(f"Added {len(pending)} file(s)."))

        self._run_in_thread(task, status="Adding files...")

    def add_folder(self):
        if not self.require_vault():
            return

        path = filedialog.askdirectory(title="Choose folder to ZIP and add")
        if not path:
            return

        folder_path = Path(path)
        # Size-based confirmation for large folders (destructive/add time safety)
        try:
            total_size = sum(f.stat().st_size for f in folder_path.rglob("*") if f.is_file())
        except Exception:
            total_size = 0
        size_mb = total_size / (1024 * 1024)

        if size_mb > 50:
            if not messagebox.askyesno(
                "Large folder add?",
                f"Folder approx {size_mb:.1f} MB.\n\n"
                "Zipping and adding may take significant time. Continue?",
            ):
                return

        zip_name = folder_path.name.rstrip("/").rstrip("\\") + ".zip"
        overwrite = True
        if zip_name in self.vault.data.get("files", {}):
            overwrite = messagebox.askyesno(
                "Overwrite existing folder entry?",
                f"'{zip_name}' already exists in the vault.\n\n"
                "Overwrite it with the new folder contents?",
            )
            if not overwrite:
                return

        self.set_status(f"Adding folder '{folder_path.name}'...")
        self._run_in_thread(lambda: self.vault.add_folder_as_zip(folder_path, overwrite=overwrite))

    def extract_selected(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return

        output_dir = filedialog.askdirectory(
            title="Choose extraction folder",
            initialdir=str(Path.cwd()),
        )
        if not output_dir:
            return

        filenames = [self.tree.item(s, "values")[0] for s in selections]
        output_path = Path(output_dir)
        existing = [fname for fname in filenames if (output_path / safe_filename(fname)).exists()]
        overwrite = False
        if existing:
            overwrite = messagebox.askyesno(
                "Overwrite on extract?",
                f"{len(existing)} file(s) already exist at the destination.\n\n"
                "Overwrite them with vault contents?\n"
                "Existing outside files will be replaced.",
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
            self.set_status(f"Extracted {len(filenames)} file(s).")
            messagebox.showinfo("Extract complete", f"Extracted {len(filenames)} file(s) to:\n{output_dir}")

        self._run_in_thread(task, done, status="Extracting files...")

    def delete_selected(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return

        filenames = [self.tree.item(s, "values")[0] for s in selections]
        names_preview = "\n".join(filenames[:5]) + ("\n..." if len(filenames) > 5 else "")
        msg = (
            f"Delete {len(filenames)} file(s) from the vault?\n\n"
            f"{names_preview}\n\n"
            "This is permanent. The encrypted entries will be removed with no recovery possible."
        )
        if not messagebox.askyesno("Confirm Delete", msg):
            return

        def done():
            self.set_status(f"Deleted {len(filenames)} file(s).")
            self.refresh_list()

        self._run_in_thread(lambda: [self.vault.delete_file(fname) for fname in filenames], done)

    def rename_selected(self):
        if not self.require_vault():
            return

        selection = self.tree.selection()
        if not selection or len(selection) > 1:
            messagebox.showwarning("Selection", "Select exactly one file to rename.")
            return

        old_name = self.tree.item(selection[0], "values")[0]
        new_name = askstring("Rename", "New filename (must be unique):", initialvalue=old_name)
        if not new_name or new_name == old_name:
            return

        self._run_in_thread(lambda: self.vault.rename_file(old_name, new_name))

    def change_password(self):
        if not self.require_vault():
            return

        if not messagebox.askyesno(
            "Confirm Password Change",
            "Change the vault password?\n\n"
            "This will re-encrypt the entire vault with the new password.\n"
            "The old password will stop working. This cannot be undone.\n\n"
            "Continue?",
        ):
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
            if not messagebox.askyesno("Continue anyway?", "Use this password despite the warning?"):
                return

        # Strong confirmation for password change (destructive re-encrypt of all entries)
        if not messagebox.askyesno(
            "Final Confirmation - Rotate Password",
            "This is your last chance.\n\n"
            "Changing the password will permanently re-encrypt every file entry.\n"
            "You will need the NEW password to unlock this vault going forward.\n\n"
            "Proceed with password change?",
        ):
            self.set_status("Password change cancelled.")
            return

        def done():
            messagebox.showinfo("Password changed", "Vault password changed and file entries re-encrypted.")
            self.set_status("Password rotated.")

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
            "Secure Open will extract plaintext copies to a temporary directory (cleaned on exit).\n\n"
            "External apps may leave caches or file history. Are you sure you want to proceed?",
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

    def open_github_releases(self):
        """Open the dedicated GitHub Releases page. Graceful fallback for headless or restricted envs.
        This directly supports users who download binaries / source from GitHub Releases.
        """
        try:
            webbrowser.open(GITHUB_RELEASES_URL)
        except Exception:
            # Fallback: show the URL so user can copy (works in tests/headless too)
            messagebox.showinfo(
                "GitHub Releases",
                f"Open in your browser:\n{GITHUB_RELEASES_URL}\n\n"
                "GitHub Releases is the dedicated area for source, wheels, checksums, and (future) binaries.\n"
                "You may see 'No releases found' until the first version tag (vX.Y.Z) is pushed.",
            )

    def show_about(self):
        about_win = ctk.CTkToplevel(self)
        about_win.title("Pulse-Vault Security Notes")
        about_win.geometry("860x620")
        about_win.resizable(False, False)

        about_win.update_idletasks()
        if self.winfo_viewable():
            x = self.winfo_x() + (self.winfo_width() // 2) - (860 // 2)
            y = self.winfo_y() + (self.winfo_height() // 2) - (620 // 2)
            about_win.geometry(f"+{x}+{y}")

        about_win.transient(self)
        about_win.grab_set()
        about_win.grid_columnconfigure(0, weight=1)
        about_win.grid_rowconfigure(1, weight=1)

        header = ctk.CTkFrame(about_win, fg_color=adaptive_color("#f0f0f0", "#1f1f1f"), corner_radius=CORNER_RADIUS_SMALL)
        header.grid(row=0, column=0, sticky="ew")
        header.grid_columnconfigure(0, weight=1)

        ctk.CTkLabel(
            header,
            text="Pulse-Vault",
            font=ctk.CTkFont(**get_ubuntu_font(28, "bold")),
            text_color=get_adaptive_accent(),  # Yaru orange accent
        ).grid(row=0, column=0, padx=30, pady=(24, 4))
        ctk.CTkLabel(
            header,
            text=f"Version {__version__} | DNSPulse hardened local vault",
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            text_color=get_adaptive_gray(),
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
            f"Official site: {OFFICIAL_SITE}"
        )

        mode = resolve_appearance_mode(ctk.get_appearance_mode())
        about_colors = get_yaru_colors(mode)
        textbox = ctk.CTkTextbox(
            about_win,
            wrap="word",
            font=ctk.CTkFont(**get_ubuntu_font(13)),
            fg_color="transparent",
            text_color=about_colors["fg"],
        )
        textbox.grid(row=1, column=0, sticky="nsew", padx=36, pady=(20, 10))
        textbox.insert("1.0", about_text)
        textbox.configure(state="disabled")

        # NEW: Dedicated GitHub downloads area inside About dialog
        downloads_frame = ctk.CTkFrame(about_win, fg_color="transparent")
        downloads_frame.grid(row=2, column=0, padx=36, pady=(0, 10), sticky="ew")
        downloads_frame.grid_columnconfigure(0, weight=1)

        ctk.CTkLabel(
            downloads_frame,
            text="GitHub Downloads & Releases",
            font=ctk.CTkFont(**get_ubuntu_font(14, "bold")),
            text_color=adaptive_color("#3b82f6", "#60a5fa"),  # external link blue distinction
        ).grid(row=0, column=0, sticky="w", pady=(0, 4))

        ctk.CTkLabel(
            downloads_frame,
            text="Download source, wheels, view release notes, and checksums. GitHub Releases is the dedicated area (may show 'No releases found' until first tag).",
            font=ctk.CTkFont(**get_ubuntu_font(11)),
            text_color=get_adaptive_gray(),
            wraplength=700,
        ).grid(row=1, column=0, sticky="w", pady=(0, 8))

        btn_row = ctk.CTkFrame(downloads_frame, fg_color="transparent")
        btn_row.grid(row=2, column=0, sticky="w")

        ctk.CTkButton(
            btn_row,
            text="Open GitHub Releases",
            command=lambda: (about_win.destroy(), self.open_github_releases()),
            fg_color=adaptive_color("#3b82f6", "#60a5fa"),
            hover_color=adaptive_color("#2563eb", "#1d4ed8"),
            height=BUTTON_HEIGHT_COMPACT,
            width=180,
            corner_radius=CORNER_RADIUS_SMALL,
        ).pack(side="left", padx=(0, 8))

        ctk.CTkButton(
            btn_row,
            text="Visit dnspulse.org",
            command=lambda: webbrowser.open(OFFICIAL_SITE) or None,
            fg_color="transparent",
            border_width=1,
            height=BUTTON_HEIGHT_COMPACT,
            corner_radius=CORNER_RADIUS_SMALL,
        ).pack(side="left")

        ctk.CTkButton(
            about_win,
            text="Close",
            command=about_win.destroy,
            width=140,
            height=BUTTON_HEIGHT_PRIMARY,
            font=ctk.CTkFont(**get_ubuntu_font(14, "bold")),
            fg_color=get_adaptive_accent(),  # Yaru orange
            hover_color=get_adaptive_accent_hover(),
            corner_radius=CORNER_RADIUS,
        ).grid(row=3, column=0, pady=(8, 28))
        about_win.focus()

    def show_downloads_dialog(self):
        """Standalone dedicated downloads dialog (can be called from menu or future features).
        Provides focused GitHub releases view for users coming from GitHub binary downloads.
        """
        dl_win = ctk.CTkToplevel(self)
        dl_win.title(f"{APP_NAME} - GitHub Releases")
        dl_win.geometry("520x280")
        dl_win.resizable(False, False)

        dl_win.update_idletasks()
        if self.winfo_viewable():
            x = self.winfo_x() + (self.winfo_width() // 2) - (dl_win.winfo_width() // 2)
            y = self.winfo_y() + (self.winfo_height() // 2) - (dl_win.winfo_height() // 2)
            dl_win.geometry(f"+{x}+{y}")

        dl_win.transient(self)
        dl_win.grab_set()
        dl_win.grid_columnconfigure(0, weight=1)

        ctk.CTkLabel(
            dl_win,
            text="GitHub Releases",
            font=ctk.CTkFont(**get_ubuntu_font(20, "bold")),
            text_color=adaptive_color("#3b82f6", "#60a5fa"),
        ).grid(row=0, column=0, padx=24, pady=(24, 8))

        info = (
            "All releases, source distributions, wheels, checksums (SHA256SUMS), "
            "and the advanced security fuzz report are published here.\n\n"
            "You may see 'No releases found' until the first `v*` tag is pushed (the release workflow then runs automatically).\n\n"
            "For packaged desktop binaries (planned toward 1.0), check GitHub Releases first. "
            "The official site (dnspulse.org) will mirror installers when ready."
        )
        ctk.CTkLabel(
            dl_win,
            text=info,
            wraplength=460,
            justify="left",
            font=ctk.CTkFont(**get_ubuntu_font(12)),
            text_color=get_adaptive_gray(),
        ).grid(row=1, column=0, padx=24, pady=(0, 16))

        btns = ctk.CTkFrame(dl_win, fg_color="transparent")
        btns.grid(row=2, column=0, padx=24, pady=(0, 20), sticky="ew")

        ctk.CTkButton(
            btns,
            text="Open Releases Page",
            command=lambda: (dl_win.destroy(), self.open_github_releases()),
            fg_color=adaptive_color("#3b82f6", "#60a5fa"),
            height=BUTTON_HEIGHT,
            corner_radius=CORNER_RADIUS_SMALL,
        ).pack(side="left", padx=(0, 10))

        ctk.CTkButton(
            btns,
            text="Close",
            command=dl_win.destroy,
            fg_color="transparent",
            border_width=1,
            height=BUTTON_HEIGHT,
            corner_radius=CORNER_RADIUS_SMALL,
        ).pack(side="left")

        dl_win.focus()