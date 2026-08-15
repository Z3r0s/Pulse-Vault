import tkinter.font as tkfont

# Ubuntu 26.04 / refreshed Yaru (GNOME 50) inspired palette for production-grade native look.
# Primary accent: Ubuntu Orange. Dark mode preferred for security tools.
# Follows Yaru direction: orange brand + upstream GNOME radii/contrast where possible.
# Expanded for more orange accents, adaptive tuples, consistent radii 6-8, button std, Ubuntu fonts.

YARU_ORANGE = "#E95420"
YARU_ORANGE_HOVER = "#C34113"
YARU_ORANGE_LIGHT = "#F37C4F"  # lighter variant for accents/hovers in light mode
YARU_GREEN = "#0e8420"
YARU_RED = "#c7162b"
YARU_SUCCESS = "#0e8420"
YARU_WARNING = "#E65100"
YARU_DANGER = "#c7162b"
YARU_BLUE_ACCENT = "#0073E5"  # fallback, prefer orange

# Consistent radii per Yaru/GNOME (6-8 for modern cards/buttons)
CORNER_RADIUS = 8
CORNER_RADIUS_SMALL = 6
CORNER_RADIUS_NONE = 0

# Standardized button heights (Ubuntu native feel)
BUTTON_HEIGHT_PRIMARY = 40
BUTTON_HEIGHT = 36
BUTTON_HEIGHT_COMPACT = 32
BUTTON_HEIGHT_SMALL = 28

# Consistent Yaru-inspired spacing (for dialogs, frames, polish)
YARU_PAD = 12
YARU_PAD_SMALL = 6
YARU_PAD_LARGE = 20


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


def adaptive_color(light: str, dark: str) -> tuple[str, str]:
    """Return CTk-compatible adaptive color tuple for light/dark modes."""
    return (light, dark)


def get_yaru_colors(mode: str) -> dict:
    """Yaru / GNOME 50 aligned colors (Ubuntu 26.04 production desktop feel).
    Expanded with more orange accent variants and Yaru-specifics."""
    if mode == "light":
        return {
            "accent": YARU_ORANGE,
            "accent_hover": YARU_ORANGE_HOVER,
            "accent_light": YARU_ORANGE_LIGHT,
            "bg": "#FAFAFA",
            "fg": "#000000",
            "card": "#FFFFFF",
            "heading_bg": "#F0F0F0",
            "heading_fg": "#3D3D3D",
            "select": YARU_ORANGE,
            "border": "#D0D0D0",
            "gray": "#666666",
            "gray_light": "#8a8a8a",
            "warning": YARU_WARNING,
            "success": YARU_SUCCESS,
            "danger": YARU_DANGER,
            "progress": YARU_ORANGE,
            "search_border": YARU_ORANGE,
            "empty_fg": "#555555",
            "empty_bg": "#FAFAFA",
        }
    return {
        "accent": YARU_ORANGE,
        "accent_hover": YARU_ORANGE_HOVER,
        "accent_light": YARU_ORANGE_LIGHT,
        "bg": "#242424",
        "fg": "#deddda",
        "card": "#2a2a2a",
        "heading_bg": "#1f1f1f",
        "heading_fg": "#deddda",
        "select": YARU_ORANGE,
        "border": "#3a3a3a",
        "gray": "#9a9996",
        "gray_light": "#b0afac",
        "warning": YARU_WARNING,
        "success": YARU_SUCCESS,
        "danger": YARU_DANGER,
        "progress": YARU_ORANGE,
        "search_border": YARU_ORANGE,
        "empty_fg": "#a0a09a",
        "empty_bg": "#242424",
    }


def tree_palette(mode: str) -> dict:
    c = get_yaru_colors(mode)
    # Yaru-refined alternating rows: subtle differentiation, Ubuntu 26.04 desktop feel
    even_bg = "#2c2c2c" if mode == "dark" else "#f0f0f0"
    return {
        "bg": c["bg"],
        "fg": c["fg"],
        "field": c["bg"],
        "heading_bg": c["heading_bg"],
        "heading_fg": c["heading_fg"],
        "select": c["select"],
        "select_fg": "#FFFFFF" if mode == "dark" else "#FFFFFF",
        "odd": c["bg"],
        "even": even_bg,
        "menu_bg": c["bg"],
        "menu_fg": c["fg"],
        "border": c["border"],
        "accent": c["accent"],
    }


def tree_fonts(root) -> tuple:
    """Return (body, heading) font tuples preferring native Ubuntu on 26.04 Yaru.
    Robust to headless/test calls (no default root)."""
    family = "Ubuntu"
    size = 11
    try:
        base = tkfont.nametofont("TkDefaultFont")
        try:
            test = tkfont.Font(family="Ubuntu", size=11)
            del test
        except Exception:
            family = base.actual("family")
        sz = base.actual("size")
        if sz:
            size = sz
    except Exception:
        # headless / early / test env
        pass
    # Slightly tuned sizes for tree readability on modern Ubuntu
    return (family, size), (family, size + 1, "bold")


def get_ubuntu_font(size: int = 13, weight: str = "normal", family: str = "Ubuntu") -> dict:
    """Helper for consistent Ubuntu font usage in CTkFont calls (fallback safe)."""
    return {"family": family, "size": size, "weight": weight}


def get_adaptive_accent() -> tuple[str, str]:
    """Primary Yaru orange accent as adaptive tuple (same brand color both modes)."""
    return adaptive_color(YARU_ORANGE, YARU_ORANGE)


def get_adaptive_accent_hover() -> tuple[str, str]:
    return adaptive_color(YARU_ORANGE_HOVER, YARU_ORANGE_HOVER)


def get_adaptive_gray() -> tuple[str, str]:
    """Subdued text colors adaptive for Yaru."""
    return adaptive_color("#666666", "#9a9996")


def get_adaptive_warning() -> tuple[str, str]:
    return adaptive_color(YARU_WARNING, "#FF8C42")


def get_adaptive_success() -> tuple[str, str]:
    return adaptive_color(YARU_SUCCESS, YARU_SUCCESS)


def get_yaru_button_colors(style: str = "primary") -> dict:
    """Return dict of fg/hover/border for buttons using Yaru orange where appropriate."""
    if style == "primary":
        return {
            "fg_color": get_adaptive_accent(),
            "hover_color": get_adaptive_accent_hover(),
        }
    if style == "danger":
        return {
            "fg_color": adaptive_color(YARU_DANGER, YARU_DANGER),
            "hover_color": adaptive_color("#a01320", "#a01320"),
        }
    if style == "warning":
        return {
            "fg_color": get_adaptive_warning(),
            "hover_color": adaptive_color("#c2410f", "#e0702e"),
        }
    # secondary / outline
    return {
        "fg_color": "transparent",
        "hover_color": adaptive_color("#e0e0e0", "#333333"),
        "border_color": adaptive_color("#c0c0c0", "#4a4a4a"),
    }


def get_dialog_colors(mode: str = "dark") -> dict:
    """Colors for polished dialogs (about, password etc) with Yaru accents."""
    c = get_yaru_colors(mode)
    return {
        "title_color": c["accent"],
        "text_color": c["fg"],
        "bg": c["card"],
        "border": c["border"],
    }


def get_progress_color() -> tuple[str, str]:
    """Orange progress bar for Yaru branding."""
    return get_adaptive_accent()


def get_search_border_color() -> tuple[str, str]:
    return adaptive_color(YARU_ORANGE, YARU_ORANGE)