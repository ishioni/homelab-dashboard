---
name: Arctic Sentinel
colors:
  surface: '#0d131e'
  surface-dim: '#0d131e'
  surface-bright: '#333945'
  surface-container-lowest: '#080e19'
  surface-container-low: '#161c27'
  surface-container: '#1a202b'
  surface-container-high: '#242a36'
  surface-container-highest: '#2f3541'
  on-surface: '#dde2f2'
  on-surface-variant: '#c0c8cb'
  inverse-surface: '#dde2f2'
  inverse-on-surface: '#2b313c'
  outline: '#8a9295'
  outline-variant: '#40484b'
  surface-tint: '#97cfe0'
  primary: '#a3dcec'
  on-primary: '#003640'
  primary-container: '#88c0d0'
  on-primary-container: '#0c4f5d'
  inverse-primary: '#2b6674'
  secondary: '#a1cfce'
  on-secondary: '#023736'
  secondary-container: '#214e4d'
  on-secondary-container: '#90bdbc'
  tertiary: '#b7d5ff'
  on-tertiary: '#033258'
  tertiary-container: '#97bae8'
  on-tertiary-container: '#254a72'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#b3ecfc'
  primary-fixed-dim: '#97cfe0'
  on-primary-fixed: '#001f26'
  on-primary-fixed-variant: '#094e5c'
  secondary-fixed: '#bdebea'
  secondary-fixed-dim: '#a1cfce'
  on-secondary-fixed: '#002020'
  on-secondary-fixed-variant: '#214e4d'
  tertiary-fixed: '#d2e4ff'
  tertiary-fixed-dim: '#a6c9f8'
  on-tertiary-fixed: '#001c37'
  on-tertiary-fixed-variant: '#234970'
  background: '#0d131e'
  on-background: '#dde2f2'
  surface-variant: '#2f3541'
typography:
  h1:
    fontFamily: Inter
    fontSize: 32px
    fontWeight: '600'
    lineHeight: '1.2'
    letterSpacing: -0.02em
  h2:
    fontFamily: Inter
    fontSize: 24px
    fontWeight: '600'
    lineHeight: '1.3'
  body-lg:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: '400'
    lineHeight: '1.5'
  body-sm:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: '400'
    lineHeight: '1.5'
  data-display:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: '500'
    lineHeight: '1.4'
  data-label:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: '500'
    lineHeight: '1.2'
    letterSpacing: 0.05em
  code-block:
    fontFamily: JetBrains Mono
    fontSize: 13px
    fontWeight: '400'
    lineHeight: '1.6'
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  base: 4px
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 40px
  gutter: 16px
  margin: 24px
---

## Brand & Style

This design system is engineered for Site Reliability Engineering (SRE) environments where clarity, precision, and cognitive ease are paramount. The brand personality is "Arctic Professional"—quietly confident, technically rigorous, and exceptionally calm under pressure. 

The visual style leverages **Minimalism** with **Tonal Layering**. It avoids unnecessary ornamentation to reduce "alert fatigue," utilizing a restricted palette to ensure that when color is used (for status or data visualization), it carries maximum intentionality. The aesthetic is clean and frost-bitten, prioritizing high-legibility and a structured interface that feels like a precision instrument.

## Colors

The color strategy uses the **Polar Night** series as the foundation for the environment, providing a deep, low-eye-strain backdrop for long-duration monitoring. 

- **Foundations:** Use `#2E3440` for primary backgrounds and `#3B4252` for elevated containers.
- **Accents:** The **Frost** palette serves as the primary action and focus series. `#88C0D0` is the primary interactive color.
- **Typography:** Use the **Snow Storm** series for text to ensure high contrast against the dark slates without the harshness of pure white.
- **Status:** Reserved "Aurora" colors (Red, Orange, Yellow, Green) are used strictly for system health indicators and critical alerts to ensure they "pop" against the otherwise cool, monochromatic UI.

## Typography

This design system employs a dual-font strategy to distinguish between UI orchestration and technical data.

- **Inter:** Used for all navigation, headers, and instructional text. It provides a neutral, modern clarity that stays out of the user's way.
- **JetBrains Mono:** Utilized for all metrics, logs, timestamps, and terminal outputs. The increased character spacing and distinct glyphs reduce errors when reading complex strings.
- **Weight Usage:** Stick to Medium (500) and SemiBold (600) for hierarchy. Regular (400) should be the standard for all body and data density.

## Layout & Spacing

The layout utilizes a **12-column fluid grid** designed for dense information displays. 

- **Grid:** Use 16px (md) gutters for standard dashboard widgets. For high-density log viewers, spacing may be reduced to 8px (sm).
- **Rhythm:** All spacing is based on a 4px baseline grid. 
- **Density:** SRE tools require high information density. Maintain a 24px (lg) margin around the main viewport to provide "visual breathing room" against the edge of the screen, while keeping internal widget padding tight (12px to 16px) to maximize data visibility.

## Elevation & Depth

Depth is achieved through **Tonal Layering** and **Low-Contrast Outlines** rather than traditional shadows. This maintains the "flat arctic" feel and avoids visual clutter.

- **Level 0 (Base):** `#2E3440` (The "frozen ground").
- **Level 1 (Card/Widget):** `#3B4252` surface with a 1px solid border of `#434C5E`.
- **Level 2 (Popovers/Modals):** `#434C5E` surface with a subtle `#4C566A` border.
- **Interactive State:** Elements like buttons or active inputs should use a subtle outer glow of the Frost primary (`#88C0D0` at 20% opacity) instead of a black shadow to simulate light reflecting off ice.

## Shapes

The design system uses a **Soft (1)** roundedness profile. This 4px (0.25rem) radius on buttons, input fields, and widgets strikes a balance between the precision of sharp corners and the modern feel of rounded UI. 

- **Small elements (Checkboxes, Tags):** 2px radius.
- **Standard elements (Buttons, Inputs, Cards):** 4px radius.
- **Large elements (Modals, Sidebars):** 8px radius.

## Components

- **Buttons:** Primary buttons use `frost-1` (#88C0D0) with `polar-night-0` text. Secondary buttons are outlined using `snow-storm-0` with no background fill.
- **Cards/Widgets:** Use `polar-night-1` backgrounds with a thin `polar-night-2` border. Headers within cards should have a subtle bottom divider to separate controls from data.
- **Status Chips:** Use a "dot + label" pattern. The dot uses the Aurora palette (e.g., `#A3BE8C` for 'Healthy'), while the chip background is a 10% opacity version of the same color.
- **Inputs:** Dark backgrounds (`#2E3440`) with `snow-storm-0` text. The bottom border or full border highlights in `frost-1` upon focus.
- **Data Tables:** Row stripes are not used. Instead, use a 1px divider in `#3B4252` and a `frost-accent` highlight on the far left of a row to indicate selection.
- **SRE Specifics:** 
    - **Log Viewer:** Monospaced font, minimized line height (1.4), and subtle syntax highlighting using the full Nord palette.
    - **KPI Sparklines:** Use `frost-2` for normal trends and `aurora-red` for anomaly spikes.