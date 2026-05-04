import os
import sys
import threading
import tempfile
import atexit
import shutil
import subprocess
from pathlib import Path
from typing import Optional, List
import tkinter as tk
from tkinter import filedialog, messagebox, ttk
from tkinter.simpledialog import askstring

import customtkinter as ctk
from tkinterdnd2 import TkinterDnD, DND_FILES

from core.vault import EncryptedVault, VaultError
from gui.dialogs import ask_password

APP_NAME = "PulseVault"
APP_VERSION = "2.0.0"

def human_size(size: int) -> str:
    units = ["B", "KB", "MB", "GB", "TB"]
    value = float(size)
    for unit in units:
        if value < 1024:
            return f"{value:.1f} {unit}"
        value /= 1024
    return f"{value:.1f} PB"

class VaultGUI(ctk.CTk):
    def __init__(self):
        super().__init__()

        self.title(f"{APP_NAME} v{APP_VERSION}")
        self.geometry("1000x650")
        self.minsize(900, 600)
        
        # Configure grid
        self.grid_columnconfigure(1, weight=1)
        self.grid_rowconfigure(0, weight=1)

        self.vault: Optional[EncryptedVault] = None
        self.filtered_files: List[str] = []
        self.secure_temp_dir = Path(tempfile.mkdtemp(prefix=".pulse_secure_"))
        atexit.register(self.cleanup_temp_dir)

        self.build_sidebar()
        self.build_main_view()
        
        # We need a progress queue/events for thread safe GUI updates
        self.bind("<<RefreshList>>", lambda _: self.refresh_list())
        self.bind("<<ClearProgress>>", lambda _: self.hide_progress())

    def cleanup_temp_dir(self):
        try:
            if self.secure_temp_dir.exists():
                shutil.rmtree(self.secure_temp_dir, ignore_errors=True)
        except Exception:
            pass

    def build_sidebar(self):
        self.sidebar_frame = ctk.CTkFrame(self, width=200, corner_radius=0)
        self.sidebar_frame.grid(row=0, column=0, rowspan=2, sticky="nsew")
        self.sidebar_frame.grid_rowconfigure(5, weight=1)

        self.logo_label = ctk.CTkLabel(self.sidebar_frame, text="PulseVault", font=ctk.CTkFont(size=20, weight="bold"))
        self.logo_label.grid(row=0, column=0, padx=20, pady=(20, 10))

        self.btn_new = ctk.CTkButton(self.sidebar_frame, text="New Vault", command=self.create_vault, fg_color="#10b981", hover_color="#059669")
        self.btn_new.grid(row=1, column=0, padx=20, pady=10)

        self.btn_open = ctk.CTkButton(self.sidebar_frame, text="Open Vault", command=self.open_vault, fg_color="#3b82f6", hover_color="#2563eb")
        self.btn_open.grid(row=2, column=0, padx=20, pady=10)

        self.btn_lock = ctk.CTkButton(self.sidebar_frame, text="Lock Vault", command=self.lock_vault, state="disabled")
        self.btn_lock.grid(row=3, column=0, padx=20, pady=10)
        
        self.btn_change_pw = ctk.CTkButton(self.sidebar_frame, text="Change Password", command=self.change_password, state="disabled", fg_color="transparent", border_width=1)
        self.btn_change_pw.grid(row=4, column=0, padx=20, pady=10)

        self.btn_about = ctk.CTkButton(self.sidebar_frame, text="About PulseVault", command=self.show_about, fg_color="transparent", border_width=1, text_color="#10b981")
        self.btn_about.grid(row=5, column=0, padx=20, pady=10)

        self.appearance_mode_label = ctk.CTkLabel(self.sidebar_frame, text="Appearance Mode:", anchor="w")
        self.appearance_mode_label.grid(row=6, column=0, padx=20, pady=(10, 0))
        self.appearance_mode_optionemenu = ctk.CTkOptionMenu(self.sidebar_frame, values=["System", "Light", "Dark"],
                                                                       command=self.change_appearance_mode_event)
        self.appearance_mode_optionemenu.grid(row=7, column=0, padx=20, pady=(10, 20))

    def build_main_view(self):
        self.main_frame = ctk.CTkFrame(self, corner_radius=0, fg_color="transparent")
        self.main_frame.grid(row=0, column=1, sticky="nsew", padx=20, pady=20)
        self.main_frame.grid_columnconfigure(0, weight=1)
        self.main_frame.grid_rowconfigure(2, weight=1)

        # Top Bar
        self.top_bar = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.top_bar.grid(row=0, column=0, sticky="ew", pady=(0, 10))
        self.top_bar.grid_columnconfigure(0, weight=1)

        self.status_label = ctk.CTkLabel(self.top_bar, text="No vault loaded.", font=ctk.CTkFont(size=14, weight="bold"))
        self.status_label.grid(row=0, column=0, sticky="w")

        self.stats_label = ctk.CTkLabel(self.top_bar, text="Files: 0 · Vault size: 0 B", text_color="gray")
        self.stats_label.grid(row=1, column=0, sticky="w")

        # Search Bar
        self.search_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.search_frame.grid(row=1, column=0, sticky="ew", pady=(0, 10))
        self.search_frame.grid_columnconfigure(0, weight=1)

        self.search_entry = ctk.CTkEntry(self.search_frame, placeholder_text="Search files...")
        self.search_entry.grid(row=0, column=0, sticky="ew", padx=(0, 10))
        self.search_entry.bind("<KeyRelease>", lambda _: self.refresh_list())

        # Progress Bar (Hidden by default)
        self.progress_bar = ctk.CTkProgressBar(self.main_frame)
        self.progress_bar.set(0)
        
        # File Tree using ttk.Treeview
        # We need a frame to hold the tree and scrollbar
        self.tree_frame = ctk.CTkFrame(self.main_frame)
        self.tree_frame.grid(row=2, column=0, sticky="nsew", pady=(0, 10))
        self.tree_frame.grid_columnconfigure(0, weight=1)
        self.tree_frame.grid_rowconfigure(0, weight=1)

        style = ttk.Style()
        style.theme_use("default")
        style.configure("Treeview", 
                        background="#2b2b2b",
                        foreground="white",
                        rowheight=30,
                        fieldbackground="#2b2b2b",
                        borderwidth=0)
        style.map('Treeview', background=[('selected', '#1f538d')])
        style.configure("Treeview.Heading",
                        background="#565b5e",
                        foreground="white",
                        relief="flat")
        style.map("Treeview.Heading",
                  background=[('active', '#3484F0')])

        columns = ("name", "size", "type")
        self.tree = ttk.Treeview(self.tree_frame, columns=columns, show="headings", selectmode="extended")
        self.tree.heading("name", text="Name")
        self.tree.heading("size", text="Size")
        self.tree.heading("type", text="Type")

        self.tree.column("name", width=400, anchor="w")
        self.tree.column("size", width=100, anchor="e")
        self.tree.column("type", width=100, anchor="center")

        yscroll = ttk.Scrollbar(self.tree_frame, orient="vertical", command=self.tree.yview)
        self.tree.configure(yscrollcommand=yscroll.set)

        self.tree.grid(row=0, column=0, sticky="nsew")
        yscroll.grid(row=0, column=1, sticky="ns")

        self.tree.bind("<Double-1>", lambda _: self.extract_selected())

        # Action Buttons
        self.action_frame = ctk.CTkFrame(self.main_frame, fg_color="transparent")
        self.action_frame.grid(row=3, column=0, sticky="ew")

        self.btn_add_file = ctk.CTkButton(self.action_frame, text="Add File", command=self.add_file, state="disabled")
        self.btn_add_file.pack(side="left", padx=(0, 10))

        self.btn_add_folder = ctk.CTkButton(self.action_frame, text="Add Folder as ZIP", command=self.add_folder, state="disabled")
        self.btn_add_folder.pack(side="left", padx=(0, 10))

        self.btn_extract = ctk.CTkButton(self.action_frame, text="Extract", command=self.extract_selected, state="disabled")
        self.btn_extract.pack(side="left", padx=(0, 10))

        self.btn_view = ctk.CTkButton(self.action_frame, text="View (Secure Open)", command=self.secure_view, state="disabled", fg_color="#f59e0b", hover_color="#d97706")
        self.btn_view.pack(side="left", padx=(0, 10))

        self.btn_delete = ctk.CTkButton(self.action_frame, text="Delete", command=self.delete_selected, state="disabled", fg_color="#ef4444", hover_color="#dc2626")
        self.btn_delete.pack(side="right")
        
        self.btn_rename = ctk.CTkButton(self.action_frame, text="Rename", command=self.rename_selected, state="disabled", fg_color="transparent", border_width=1)
        self.btn_rename.pack(side="right", padx=(0, 10))

    def change_appearance_mode_event(self, new_appearance_mode: str):
        ctk.set_appearance_mode(new_appearance_mode)

    def set_status(self, message: str):
        self.status_label.configure(text=message)

    def update_stats(self):
        if not self.vault or not self.vault.is_unlocked:
            self.stats_label.configure(text="Files: 0 · Vault size: 0 B")
            return

        try:
            stats = self.vault.stats()
            if self.vault.version == 3:
                v = "v3 (Cascade + Scrypt)"
            elif self.vault.version == 2:
                v = "v2 (Streaming)"
            else:
                v = "v1 (Legacy)"
                
            self.stats_label.configure(
                text=f"Files: {stats['file_count']} · "
                     f"Vault size: {human_size(stats['vault_disk_size'])} · "
                     f"Format: {v}"
            )
        except Exception:
            self.stats_label.configure(text="Stats unavailable.")

    def update_button_states(self, unlocked: bool):
        state = "normal" if unlocked else "disabled"
        self.btn_lock.configure(state=state)
        self.btn_change_pw.configure(state=state)
        self.btn_add_file.configure(state=state)
        self.btn_add_folder.configure(state=state)
        self.btn_extract.configure(state=state)
        self.btn_view.configure(state=state)
        self.btn_delete.configure(state=state)
        self.btn_rename.configure(state=state)

    def require_vault(self) -> bool:
        if not self.vault or not self.vault.is_unlocked:
            messagebox.showwarning("No vault", "Create or open a vault first.")
            return False
        return True

    def show_progress(self):
        self.progress_bar.grid(row=1, column=0, sticky="ew", pady=(10, 0))
        self.progress_bar.set(0)
        self.search_frame.grid_forget()

    def hide_progress(self):
        self.progress_bar.grid_forget()
        self.search_frame.grid(row=1, column=0, sticky="ew", pady=(0, 10))

    def update_progress(self, current: int, total: int):
        if total > 0:
            self.progress_bar.set(current / total)

    def refresh_list(self):
        for item in self.tree.get_children():
            self.tree.delete(item)

        self.filtered_files = []

        if not self.vault or not self.vault.is_unlocked:
            self.update_stats()
            return

        query = self.search_entry.get().strip().lower()

        for filename in self.vault.list_files():
            if query and query not in filename.lower():
                continue

            try:
                meta = self.vault.get_file_meta(filename)
            except Exception:
                continue

            file_type = meta.get("type", "file")
            size = int(meta.get("size", 0))

            self.tree.insert("", "end", values=(filename, human_size(size), file_type))
            self.filtered_files.append(filename)

        self.update_stats()

    def create_vault(self):
        path = filedialog.asksaveasfilename(
            title="Create encrypted vault",
            defaultextension=".PulseVault",
            filetypes=[("PulseVault files", "*.PulseVault"), ("Image/Video (Steganography)", "*.png *.mp4 *.jpg"), ("All files", "*.*")],
        )

        if not path:
            return

        carrier_path = None
        if messagebox.askyesno("Steganography", "Do you want to disguise this vault by hiding it inside an image or video file?"):
            carrier = filedialog.askopenfilename(title="Select Carrier Image/Video", filetypes=[("Media files", "*.png *.mp4 *.jpg")])
            if carrier:
                carrier_path = Path(carrier)

        password = ask_password(self, "Create Vault Password", confirm=True, show_generate=True)
        if not password:
            return

        try:
            vault = EncryptedVault(Path(path))
            vault.create(password, carrier_path=carrier_path)
            self.vault = vault
            self.set_status(f"Unlocked: {Path(path).name}")
            self.update_button_states(True)
            self.refresh_list()
        except Exception as e:
            self.vault = None
            self.refresh_list()
            messagebox.showerror("Error", str(e))

    def open_vault(self):
        path = filedialog.askopenfilename(
            title="Open encrypted vault",
            filetypes=[("PulseVault files", "*.PulseVault"), ("All files", "*.*")],
        )

        if not path:
            return

        self.auto_open_vault(path)

    def auto_open_vault(self, path: str):
        password = ask_password(self, "Unlock Vault")
        if not password:
            return

        try:
            target_path = Path(path)
            vault = EncryptedVault(target_path)
            vault.unlock(password)
            
            # Auto-rename .vault to .PulseVault
            if target_path.suffix == ".vault":
                new_path = target_path.with_suffix(".PulseVault")
                if not new_path.exists():
                    target_path.rename(new_path)
                    vault.vault_path = new_path
                    vault.save() # Ensures internals point correctly
                    target_path = new_path
            
            self.vault = vault
            self.set_status(f"Unlocked: {target_path.name}")
            self.update_button_states(True)
            self.refresh_list()
        except Exception as e:
            self.vault = None
            self.refresh_list()
            messagebox.showerror("Unlock failed", str(e))

    def lock_vault(self):
        if self.vault:
            self.vault.lock()

        self.vault = None
        self.search_entry.delete(0, 'end')
        self.set_status("Vault locked.")
        self.update_button_states(False)
        self.refresh_list()

    def _run_in_thread(self, task_func, on_complete=None):
        """Runs a task in a separate thread to keep UI responsive"""
        self.update_button_states(False)
        self.show_progress()
        self.progress_bar.configure(mode="indeterminate")
        self.progress_bar.start()

        def wrapper():
            error = None
            try:
                task_func()
            except Exception as e:
                error = e
            
            # Update GUI back in main thread
            self.after(0, self._thread_complete, error, on_complete)

        threading.Thread(target=wrapper, daemon=True).start()

    def _thread_complete(self, error, on_complete):
        self.progress_bar.stop()
        self.progress_bar.configure(mode="determinate")
        self.event_generate("<<ClearProgress>>")
        self.update_button_states(True)
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

        def task():
            for raw_path in paths:
                path = Path(raw_path)
                overwrite = False
                if path.name in self.vault.data.get("files", {}):
                    # Show warning in main thread, thread will block
                    # Since we are in a thread, messagebox might have issues, 
                    # but for now we'll assume it's okay or just skip. 
                    # For a robust solution we'd need thread-safe prompts.
                    # We will just overwrite for simplicity in thread, or skip.
                    # Best approach: skip if exists, user must delete first.
                    continue
                
                self.vault.add_file(path, overwrite=True)

        self._run_in_thread(task)

    def add_folder(self):
        if not self.require_vault():
            return

        path = filedialog.askdirectory(title="Choose folder to ZIP and add")
        if not path:
            return

        def task():
            self.vault.add_folder_as_zip(Path(path), overwrite=True)

        self._run_in_thread(task)

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

        def task():
            for fname in filenames:
                self.vault.extract_file(fname, Path(output_dir), overwrite=True)

        def done():
            messagebox.showinfo("Extracted", f"Extracted {len(filenames)} file(s) to:\n{output_dir}")

        self._run_in_thread(task, done)

    def delete_selected(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return

        filenames = [self.tree.item(s, "values")[0] for s in selections]

        confirm = messagebox.askyesno("Delete", f"Delete {len(filenames)} file(s)?")
        if not confirm:
            return

        def task():
            for fname in filenames:
                self.vault.delete_file(fname)
        
        self._run_in_thread(task)

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

        def task():
            self.vault.rename_file(old_name, new_name)

        self._run_in_thread(task)

    def change_password(self):
        if not self.require_vault():
            return

        old_pw = ask_password(self, "Current Password")
        if not old_pw: return

        new_pw = ask_password(self, "New Password", confirm=True)
        if not new_pw: return

        def task():
            self.vault.change_password(old_pw, new_pw)

        def done():
            messagebox.showinfo("Password changed", "Vault password changed successfully.")

        self._run_in_thread(task, done)

    def secure_view(self):
        if not self.require_vault():
            return

        selections = self.tree.selection()
        if not selections:
            return
            
        filenames = [self.tree.item(s, "values")[0] for s in selections]

        def task():
            for fname in filenames:
                output_path = self.vault.extract_file(fname, self.secure_temp_dir, overwrite=True)
                
                # Open the file via OS
                if os.name == 'nt': # Windows
                    os.startfile(output_path)
                elif sys.platform == 'darwin': # macOS
                    subprocess.call(('open', output_path))
                else: # Linux / Parrot
                    subprocess.call(('xdg-open', output_path))

        self._run_in_thread(task)

    def show_about(self):
        about_win = ctk.CTkToplevel(self)
        about_win.title("About PulseVault")
        about_win.geometry("750x500")
        about_win.resizable(False, False)
        
        # Center the dialog on the parent window
        about_win.update_idletasks()
        if self.winfo_viewable():
            x = self.winfo_x() + (self.winfo_width() // 2) - (about_win.winfo_width() // 2)
            y = self.winfo_y() + (self.winfo_height() // 2) - (about_win.winfo_height() // 2)
            about_win.geometry(f"+{x}+{y}")

        about_win.transient(self)
        about_win.grab_set()

        about_win.grid_columnconfigure(0, weight=1)
        about_win.grid_rowconfigure(0, weight=1)

        about_text = (
            "💀 PulseVault by z3r0s (DNSPulse) 💀\n\n"
            "This isn't your standard encrypted folder. This is a paranoid-level, zero-trust cryptographic fortress "
            "designed specifically for high-risk, hostile environments.\n\n"
            "Unlike default Kali Linux or Parrot OS vaults, PulseVault deploys a Custom Cascading Cipher Suite (PULSEVAULT3). "
            "Every file is independently encrypted twice: first with ChaCha20-Poly1305, and then entirely encapsulated in AES-256-GCM. "
            "Even if a 0-day is discovered in one algorithm, the secondary layer holds the line.\n\n"
            "Key Derivation is powered by memory-hard SCRYPT, completely neutering GPU cluster brute-forcing. "
            "Unencrypted buffers are systematically purged from RAM. Extraction uses secure, hidden memory spaces.\n\n"
            "We take ultimate pride in our encryption architectures. We are always evolving, always adapting, and relentlessly making it better. "
            "Your security is an arms race, and we intend to win it.\n\n"
            "No telemetry. No networking. No APIs. Pure Cryptography."
        )

        textbox = ctk.CTkTextbox(about_win, wrap="word", font=ctk.CTkFont(size=15), fg_color="transparent")
        textbox.grid(row=0, column=0, sticky="nsew", padx=40, pady=(40, 20))
        textbox.insert("1.0", about_text)
        textbox.configure(state="disabled")

        btn_close = ctk.CTkButton(about_win, text="Close", command=about_win.destroy, width=140, height=40, font=ctk.CTkFont(size=14, weight="bold"))
        btn_close.grid(row=1, column=0, pady=(0, 30))
        
        about_win.focus()
