import tkinter as tk
from tkinter import messagebox
import customtkinter as ctk

class PasswordDialog(ctk.CTkToplevel):
    def __init__(self, parent, title: str, confirm: bool = False, show_generate: bool = False):
        super().__init__(parent)
        
        self.result = None
        self.confirm = confirm

        self.title(title)
        self.geometry("380x300" if confirm else "380x250")
        self.resizable(False, False)
        
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

        self.password_entry = ctk.CTkEntry(self, show="*", placeholder_text="Password", width=340)
        self.password_entry.grid(row=1, column=0, padx=20, pady=(0, 10))

        if confirm:
            self.confirm_entry = ctk.CTkEntry(self, show="*", placeholder_text="Confirm Password", width=340)
            self.confirm_entry.grid(row=2, column=0, padx=20, pady=(0, 10))

        # Buttons
        button_frame = ctk.CTkFrame(self, fg_color="transparent")
        button_frame.grid(row=3 if confirm else 2, column=0, padx=20, pady=(10, 20), sticky="ew")

        if show_generate:
            import secrets
            def generate_key():
                import secrets
                from pathlib import Path
                key = secrets.token_hex(24)
                self.password_entry.delete(0, 'end')
                self.password_entry.insert(0, key)
                self.password_entry.configure(show="")
                if confirm:
                    self.confirm_entry.delete(0, 'end')
                    self.confirm_entry.insert(0, key)
                    self.confirm_entry.configure(show="")
                # Save to home directory so it's always findable and never accidentally committed
                recovery_path = Path.home() / ".pulse_vault_recovery_keys.txt"
                with open(recovery_path, "a") as f:
                    import datetime
                    f.write(f"[{datetime.datetime.now().isoformat()}]  {key}\n")
                messagebox.showinfo(
                    "Key Generated",
                    f"A secure key was generated and saved to:\n{recovery_path}\n\nStore this somewhere safe!",
                    parent=self
                )

            gen_btn = ctk.CTkButton(button_frame, text="Auto-Generate Key", width=120, fg_color="#8b5cf6", hover_color="#7c3aed", command=generate_key)
            gen_btn.pack(side="left")

        cancel_btn = ctk.CTkButton(button_frame, text="Cancel", width=80, fg_color="transparent", border_width=1, text_color=("gray10", "#DCE4EE"), command=self.cancel)
        cancel_btn.pack(side="right", padx=(10, 0))

        ok_btn = ctk.CTkButton(button_frame, text="OK", width=80, command=self.ok)
        ok_btn.pack(side="right")

        self.password_entry.focus()
        self.bind("<Return>", lambda _: self.ok())
        self.bind("<Escape>", lambda _: self.cancel())

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
