"""GUI package.

Heavy GUI dependencies (customtkinter, tkinterdnd2) live behind the optional
``pulse-vault[gui]`` extra. Keep this package init lightweight so submodules
like ``theme`` can be imported without requiring those extras (e.g. unit tests,
metadata tooling). ``VaultGUI`` is exported lazily.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

__all__ = ["VaultGUI"]

if TYPE_CHECKING:
    from .app import VaultGUI as VaultGUI


def __getattr__(name: str) -> Any:
    if name == "VaultGUI":
        from .app import VaultGUI as _VaultGUI

        return _VaultGUI
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
