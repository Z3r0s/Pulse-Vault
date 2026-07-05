import secrets
import tkinter as tk
from tkinter import messagebox
import customtkinter as ctk


def password_strength(password: str) -> tuple[str, str]:
    score = 0
    if len(password) >= 14:
        score += 1
    if len(password) >= 18:
        score += 1
    if any(ch.islower() for ch in password) and any(ch.isupper() for ch in password):
        score += 1
    if any(ch.isdigit() for ch in password):
        score += 1
    if any(not ch.isalnum() for ch in password):
        score += 1
    if len(set(password)) >= 8:
        score += 1

    if score >= 6:
        return "Strong", "#10b981"
    if score >= 4:
        return "Moderate", "#f59e0b"
    return "Weak", "#ef4444"

class PasswordDialog(ctk.CTkToplevel):
    def __init__(self, parent, title: str, confirm: bool = False, show_generate: bool = False):
        super().__init__(parent)
        
        self.result = None
        self.confirm = confirm

        self.title(title)
        self.geometry("430x360" if confirm else "430x310")
        self.resizable(False, False)
        self.show_password = False
        
        # Center the dialog on the parent window
        self.update_idletasks()
        if parent.winfo_viewable():
            x = parent.winfo_x() + (parent.winfo_width() // 2) - (self.winfo_width() // 2)
            y = parent.winfo_y() + (parent.winfo_height() // 2) - (self.winfo_height() // 2)
            self.geometry(f"+{x}+{y}")

        self.transient(parent)
        self.grab_set()

        self.protocol("WM_DELETE_WINDOW", self.cancel)

        # Layout
        self.grid_columnconfigure(0, weight=1)

        title_label = ctk.CTkLabel(self, text=title, font=ctk.CTkFont(size=18, weight="bold"))
        title_label.grid(row=0, column=0, padx=20, pady=(20, 10), sticky="w")

        self.password_entry = ctk.CTkEntry(self, show="*", placeholder_text="Password", width=390)
        self.password_entry.grid(row=1, column=0, padx=20, pady=(0, 8))
        self.password_entry.bind("<KeyRelease>", lambda _: self.update_strength())

        if confirm:
            self.confirm_entry = ctk.CTkEntry(self, show="*", placeholder_text="Confirm Password", width=390)
            self.confirm_entry.grid(row=2, column=0, padx=20, pady=(0, 8))

        self.strength_label = ctk.CTkLabel(
            self,
            text="Strength: Weak",
            anchor="w",
            font=ctk.CTkFont(size=11, weight="bold"),
            text_color="#ef4444",
        )
        self.strength_label.grid(row=3 if confirm else 2, column=0, padx=20, pady=(0, 8), sticky="ew")

        # Buttons
        button_frame = ctk.CTkFrame(self, fg_color="transparent")
        button_frame.grid(row=4 if confirm else 3, column=0, padx=20, pady=(8, 20), sticky="ew")

        if show_generate:
            def generate_key():
                key = secrets.token_urlsafe(32)
                self.password_entry.delete(0, 'end')
                self.password_entry.insert(0, key)
                if confirm:
                    self.confirm_entry.delete(0, 'end')
                    self.confirm_entry.insert(0, key)
                self.show_password = True
                self.apply_show_state()
                self.update_strength()
                messagebox.showinfo(
                    "Key Generated",
                    "A secure key was generated and filled into the password fields.\n\n"
                    "Pulse-Vault will not save this key for you. Store it somewhere safe before closing this dialog.",
                    parent=self
                )

            gen_btn = ctk.CTkButton(button_frame, text="Auto-Generate Key", width=120, fg_color="#8b5cf6", hover_color="#7c3aed", command=generate_key)
            gen_btn.pack(side="left")

        show_btn = ctk.CTkButton(
            button_frame,
            text="Show",
            width=70,
            fg_color="transparent",
            border_width=1,
            command=self.toggle_show_password,
        )
        show_btn.pack(side="left", padx=(10, 0))
        self.show_btn = show_btn

        cancel_btn = ctk.CTkButton(button_frame, text="Cancel", width=80, fg_color="transparent", border_width=1, text_color=("gray10", "#DCE4EE"), command=self.cancel)
        cancel_btn.pack(side="right", padx=(10, 0))

        ok_btn = ctk.CTkButton(button_frame, text="OK", width=80, command=self.ok)
        ok_btn.pack(side="right")

        self.password_entry.focus()
        self.bind("<Return>", lambda _: self.ok())
        self.bind("<Escape>", lambda _: self.cancel())
        self.update_strength()

    def apply_show_state(self):
        show_char = "" if self.show_password else "*"
        self.password_entry.configure(show=show_char)
        if self.confirm:
            self.confirm_entry.configure(show=show_char)
        self.show_btn.configure(text="Hide" if self.show_password else "Show")

    def toggle_show_password(self):
        self.show_password = not self.show_password
        self.apply_show_state()

    def update_strength(self):
        label, color = password_strength(self.password_entry.get())
        self.strength_label.configure(text=f"Strength: {label}", text_color=color)

    def ok(self):
        password = self.password_entry.get()

        if not password:
            messagebox.showerror("Missing password", "Password cannot be empty.", parent=self)
            return

        if self.confirm:
            confirm_value = self.confirm_entry.get()
            if password != confirm_value:
                messagebox.showerror("Password mismatch", "Passwords do not match.", parent=self)
                return

        self.result = password
        self.destroy()

    def cancel(self):
        self.result = None
        self.destroy()

def ask_password(parent, title: str, confirm: bool = False, show_generate: bool = False):
    dialog = PasswordDialog(parent, title=title, confirm=confirm, show_generate=show_generate)
    parent.wait_window(dialog)
    return dialog.result


class ScryptProfileDialog(ctk.CTkToplevel):
    def __init__(self, parent):
        super().__init__(parent)
        self.result = None
        self.title("Key Derivation Strength")
        self.geometry("460x300")
        self.resizable(False, False)
        self.update_idletasks()
        if parent.winfo_viewable():
            x = parent.winfo_x() + (parent.winfo_width() // 2) - (self.winfo_width() // 2)
            y = parent.winfo_y() + (parent.winfo_height() // 2) - (self.winfo_height() // 2)
            self.geometry(f"+{x}+{y}")

        self.transient(parent)
        self.grab_set()
        self.protocol("WM_DELETE_WINDOW", self.cancel)

        self.grid_columnconfigure(0, weight=1)

        title_label = ctk.CTkLabel(
            self,
            text="Choose Scrypt strength",
            font=ctk.CTkFont(size=18, weight="bold"),
        )
        title_label.grid(row=0, column=0, padx=20, pady=(20, 8), sticky="w")

        help_label = ctk.CTkLabel(
            self,
            text="Stronger settings slow down password guessing but also make unlock slower.",
            wraplength=400,
            justify="left",
        )
        help_label.grid(row=1, column=0, padx=20, pady=(0, 12), sticky="w")

        self.profile_var = tk.StringVar(value="standard")
        standard = ctk.CTkRadioButton(
            self,
            text="Standard (recommended)",
            variable=self.profile_var,
            value="standard",
        )
        standard.grid(row=2, column=0, padx=20, pady=4, sticky="w")

        hardened = ctk.CTkRadioButton(
            self,
            text="Hardened (slower unlock, higher guessing cost)",
            variable=self.profile_var,
            value="hardened",
        )
        hardened.grid(row=3, column=0, padx=20, pady=4, sticky="w")

        button_frame = ctk.CTkFrame(self, fg_color="transparent")
        button_frame.grid(row=4, column=0, padx=20, pady=(18, 20), sticky="e")

        cancel_btn = ctk.CTkButton(
            button_frame,
            text="Cancel",
            width=80,
            fg_color="transparent",
            border_width=1,
            text_color=("gray10", "#DCE4EE"),
            command=self.cancel,
        )
        cancel_btn.pack(side="right", padx=(10, 0))

        ok_btn = ctk.CTkButton(button_frame, text="Continue", width=90, command=self.ok)
        ok_btn.pack(side="right")

        self.bind("<Escape>", lambda _: self.cancel())

    def ok(self):
        self.result = self.profile_var.get()
        self.destroy()

    def cancel(self):
        self.result = None
        self.destroy()


def ask_scrypt_profile(parent):
    dialog = ScryptProfileDialog(parent)
    parent.wait_window(dialog)
    return dialog.result


# NEW: Lightweight GitHub Releases info dialog class (used optionally; main integration in app.py for simplicity)
class GitHubReleasesDialog(ctk.CTkToplevel):
    """Small dedicated dialog exposing the GitHub downloads / releases area.
    Intended for direct use or future menu integration. Keeps download goals visible.
    """
    def __init__(self, parent, releases_url: str, site_url: str = "https://dnspulse.org"):
        super().__init__(parent)
        self.releases_url = releases_url
        self.site_url = site_url
        self.title("GitHub Releases")
        self.geometry("480x220")
        self.resizable(False, False)
        self.update_idletasks()
        if parent and parent.winfo_viewable():
            x = parent.winfo_x() + (parent.winfo_width() // 2) - (self.winfo_width() // 2)
            y = parent.winfo_y() + (parent.winfo_height() // 2) - (self.winfo_height() // 2)
            self.geometry(f"+{x}+{y}")
        self.transient(parent)
        self.grab_set()
        self.protocol("WM_DELETE_WINDOW", self.destroy)
        self.grid_columnconfigure(0, weight=1)

        ctk.CTkLabel(
            self,
            text="Pulse-Vault GitHub Releases",
            font=ctk.CTkFont(size=16, weight="bold"),
            text_color="#3b82f6",
        ).grid(row=0, column=0, padx=20, pady=(18, 6))

        desc = ctk.CTkLabel(
            self,
            text="Source, wheels, checksums, and release notes are published on GitHub.\nPackaged desktop downloads will appear here (and mirrored on dnspulse.org).",
            wraplength=420,
            justify="left",
            font=ctk.CTkFont(size=11),
        )
        desc.grid(row=1, column=0, padx=20, pady=(0, 12))

        btn_frame = ctk.CTkFrame(self, fg_color="transparent")
        btn_frame.grid(row=2, column=0, padx=20, pady=(0, 18), sticky="ew")

        ctk.CTkButton(
            btn_frame,
            text="Open Releases",
            command=self._open_releases,
            fg_color="#3b82f6",
            height=32,
        ).pack(side="left", padx=(0, 8))

        ctk.CTkButton(
            btn_frame,
            text="Close",
            command=self.destroy,
            fg_color="transparent",
            border_width=1,
            height=32,
        ).pack(side="left")

    def _open_releases(self):
        import webbrowser
        try:
            webbrowser.open(self.releases_url)
        except Exception:
            pass  # parent can show fallback
        self.destroy()


def show_github_releases_dialog(parent, releases_url: str):
    """Helper to instantiate the new dedicated dialog class."""
    dlg = GitHubReleasesDialog(parent, releases_url)
    parent.wait_window(dlg)