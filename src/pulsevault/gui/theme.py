import tkinter.font as tkfont


def resolve_appearance_mode(ctk_mode: str) -> str:
    if ctk_mode == "Light":
        return "light"
    if ctk_mode == "Dark":
        return "dark"
    try:
        import darkdetect
        return "dark" if darkdetect.isDark() else "light"
    except Exception:
        return "dark"


def tree_palette(mode: str) -> dict:
    if mode == "light":
        return {
            "bg": "#f8fafc",
            "fg": "#0f172a",
            "field": "#f8fafc",
            "heading_bg": "#e2e8f0",
            "heading_fg": "#047857",
            "select": "#2563eb",
            "odd": "#f8fafc",
            "even": "#f1f5f9",
            "menu_bg": "#f8fafc",
            "menu_fg": "#0f172a",
        }
    return {
        "bg": "#111827",
        "fg": "#e5e7eb",
        "field": "#111827",
        "heading_bg": "#0f172a",
        "heading_fg": "#a7f3d0",
        "select": "#1d4ed8",
        "odd": "#111827",
        "even": "#182235",
        "menu_bg": "#111827",
        "menu_fg": "white",
    }


def tree_fonts(root) -> tuple:
    base = tkfont.nametofont("TkDefaultFont")
    family = base.actual("family")
    size = base.actual("size")
    return (family, size), (family, size, "bold")