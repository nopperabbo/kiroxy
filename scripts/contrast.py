#!/usr/bin/env python3
"""
kiroxy contrast audit — reproduces docs/DESIGN_TOKENS_AUDIT.md.

Usage: python3 scripts/contrast.py

Source of truth for token values: internal/server/assets/tokens/tokens.css.
When a token changes, update the TOKENS dict below AND regenerate the audit
document. A CI hook (planned for v1.3) will fail when ratios regress below
the committed baseline.

Methodology:
    OKLCH --> Oklab --> linear sRGB --> gamma-corrected sRGB (0..1)
    WCAG 2.2 relative-luminance contrast ratio:
        L = 0.2126*R + 0.7152*G + 0.0722*B  (where R,G,B are linearized)
        ratio = (max(L1,L2)+0.05) / (min(L1,L2)+0.05)

Thresholds (WCAG 2.2):
    1.4.3 normal text: >= 4.5 (AA), >= 7.0 (AAA)
    1.4.3 large text:  >= 3.0 (AA), >= 4.5 (AAA)
    1.4.11 non-text:   >= 3.0

Numbers match WebAIM within +-0.02 (rounding).
"""
from __future__ import annotations

import math
import sys
from typing import Dict, Tuple

Color = Tuple[float, float, float]  # sRGB in 0..1


def _oklab_to_linear_srgb(L: float, a: float, b: float) -> Color:
    l_ = L + 0.3963377774 * a + 0.2158037573 * b
    m_ = L - 0.1055613458 * a - 0.0638541728 * b
    s_ = L - 0.0894841775 * a - 1.2914855480 * b
    l, m, s = l_ ** 3, m_ ** 3, s_ ** 3
    r = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s
    g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s
    bl = -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s
    return (r, g, bl)


def _gamma(c: float) -> float:
    if c <= 0.0031308:
        c = 12.92 * c
    else:
        c = 1.055 * (c ** (1 / 2.4)) - 0.055
    return max(0.0, min(1.0, c))


def oklch_to_srgb(L: float, C: float, h_deg: float) -> Color:
    h = math.radians(h_deg)
    a = C * math.cos(h)
    b = C * math.sin(h)
    r, g, bl = _oklab_to_linear_srgb(L, a, b)
    return (_gamma(r), _gamma(g), _gamma(bl))


def hex_of(c: Color) -> str:
    r, g, b = c
    return "#{:02X}{:02X}{:02X}".format(
        int(round(r * 255)), int(round(g * 255)), int(round(b * 255))
    )


def rel_luminance(c: Color) -> float:
    def lin(x: float) -> float:
        return x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4
    r, g, b = c
    return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)


def contrast(c1: Color, c2: Color) -> float:
    L1, L2 = rel_luminance(c1), rel_luminance(c2)
    if L1 < L2:
        L1, L2 = L2, L1
    return (L1 + 0.05) / (L2 + 0.05)


def rate(ratio: float, *, non_text: bool = False) -> str:
    if non_text:
        return "AA" if ratio >= 3.0 else "FAIL"
    if ratio >= 7.0:
        return "AAA"
    if ratio >= 4.5:
        return "AA"
    if ratio >= 3.0:
        return "AA-large"
    return "FAIL"


# --- TOKEN VALUES (mirrored from tokens.css) ----------------------------------

TOKENS: Dict[str, Dict[str, Tuple[float, float, float]]] = {
    "DARK": {
        "bg":           (0.145, 0.005, 285),
        "surface":      (0.205, 0.006, 285),
        "elevated":     (0.265, 0.007, 285),
        "border":       (0.340, 0.008, 285),
        "text-dim":     (0.660, 0.015, 285),
        "text-default": (0.830, 0.012, 285),
        "text-bright":  (0.970, 0.003, 285),
        "accent":       (0.720, 0.130, 200),
        "success":      (0.720, 0.180, 145),
        "warning":      (0.800, 0.165,  85),
        "danger":       (0.680, 0.220,  25),
        "info":         (0.720, 0.130, 240),
    },
    "DARK-DIMMED": {
        "bg":           (0.195, 0.008, 285),
        "surface":      (0.255, 0.008, 285),
        "elevated":     (0.315, 0.008, 285),
        "border":       (0.380, 0.009, 285),
        "text-dim":     (0.660, 0.015, 285),
        "text-default": (0.830, 0.012, 285),
        "text-bright":  (0.970, 0.003, 285),
        "accent":       (0.720, 0.130, 200),
        "success":      (0.720, 0.180, 145),
        "warning":      (0.800, 0.165,  85),
        "danger":       (0.680, 0.220,  25),
        "info":         (0.720, 0.130, 240),
    },
    "LIGHT": {
        "bg":           (0.995, 0.000,   0),
        "surface":      (0.975, 0.002, 285),
        "elevated":     (0.945, 0.004, 285),
        "border":       (0.895, 0.005, 285),
        "text-dim":     (0.500, 0.015, 285),
        "text-default": (0.180, 0.010, 285),
        "text-bright":  (0.080, 0.005, 285),
        "accent":       (0.500, 0.155, 200),
        "success":      (0.500, 0.155, 145),
        "warning":      (0.540, 0.170,  60),
        "danger":       (0.520, 0.220,  25),
        "info":         (0.500, 0.180, 240),
    },
    "DARK-HIGHCONTRAST": {
        "bg":           (0.080, 0.000,   0),
        "surface":      (0.150, 0.000,   0),
        "elevated":     (0.220, 0.000,   0),
        "border":       (0.550, 0.000,   0),
        "text-dim":     (0.800, 0.010, 285),
        "text-default": (0.950, 0.005, 285),
        "text-bright":  (1.000, 0.000,   0),
        "accent":       (0.820, 0.150, 200),
    },
    "LIGHT-HIGHCONTRAST": {
        "bg":           (1.000, 0.000,   0),
        "surface":      (0.960, 0.000,   0),
        "elevated":     (0.920, 0.000,   0),
        "border":       (0.350, 0.000,   0),
        "text-dim":     (0.250, 0.000,   0),
        "text-default": (0.100, 0.000,   0),
        "text-bright":  (0.000, 0.000,   0),
        "accent":       (0.350, 0.200, 240),
    },
}

# (fg, bg, is_non_text)
PAIRS = [
    ("text-default", "bg", False),
    ("text-default", "surface", False),
    ("text-default", "elevated", False),
    ("text-dim",     "bg", False),
    ("text-dim",     "surface", False),
    ("text-bright",  "bg", False),
    ("text-bright",  "surface", False),
    ("accent",       "bg", False),
    ("accent",       "surface", False),
    ("success",      "bg", False),
    ("warning",      "bg", False),
    ("danger",       "bg", False),
    ("info",         "bg", False),
    ("border",       "bg", True),
    ("border",       "surface", True),
]


def main() -> int:
    failures = 0
    for theme, tokens in TOKENS.items():
        print(f"\n## {theme}")
        print(f"{'Pair':40s} {'fg':9s} {'bg':9s} {'ratio':>6s}  rating")
        print("-" * 80)
        for fg, bg, non_text in PAIRS:
            if fg not in tokens or bg not in tokens:
                continue
            c1 = oklch_to_srgb(*tokens[fg])
            c2 = oklch_to_srgb(*tokens[bg])
            r = contrast(c1, c2)
            label = rate(r, non_text=non_text)
            print(f"{fg+' on '+bg:40s} {hex_of(c1):9s} {hex_of(c2):9s} {r:6.2f}  {label}")
            if label == "FAIL" and not (fg == "border" and theme in ("DARK", "DARK-DIMMED", "LIGHT")):
                # Default-theme border is a documented exception (DESIGN_TOKENS_AUDIT.md section 1)
                failures += 1
    if failures:
        print(f"\nFAIL: {failures} unexpected WCAG failures", file=sys.stderr)
        return 1
    print("\nOK: all non-exception pairs pass their WCAG threshold")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
