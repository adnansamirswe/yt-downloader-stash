# DESIGN.md — Stash Visual Identity & Interface System

> Source of truth for all UI, layout, typography, color, motion, and accessibility decisions in **Stash**.

## 1. Audience & Context of Use

Creators, researchers, DJs, and power users archiving YouTube videos, audio tracks, and selective playlists on desktop and mobile. Stash is opened for fast, focused utility sessions: paste a link, confirm format or pick specific tracks from a playlist, watch precise live progress, and retrieve clean files without distractions, ads, or visual bloat.

## 2. Concept

**"An archival tape mastering console and vinyl catalog sheet."**

## 3. Three References

1. **Braun Atelier / Dieter Rams hi-fi decks** — Matte warm stone faceplates, 1px structural hairlines, tactile mechanical segmented switches, and a single restrained vermilion indicator reserved strictly for active capture and primary action.
2. **Criterion Collection & ECM Records catalog sheets** — Refined editorial serif display titles paired with disciplined monospace catalog numbers, durations, bitrates, and codec specs aligned with `tabular-nums`.
3. **Linear / Raycast intake ergonomics** — Keyboard-first command intake (`⌘V` / `Ctrl+V`), instant filtering, `Shift+Click` contiguous range selection in dense lists, and a non-blocking active transfers drawer.

## 4. Signature Move (One Ownable Element, Visible Above the Fold)

**The Precision Media Intake Bar & Live Telemetry Tape Ruler:**
A full-width architectural hairline console bar featuring automatic source detection (`VIDEO` / `PLAYLIST` badge), an integrated mechanical `Video | Audio` segmented switch with a crisp sliding ink indicator, and — during inspection or active transfer — a **2px signal-vermilion hairline ruler + live `IBM Plex Mono` telemetry strip** (`1080p · H.264 · REMUX · 14.8 MB/s · 00:08`) sweeping smoothly across the top edge of the active media deck or individual playlist track row. Under `prefers-reduced-motion: reduce`, the sweep degrades cleanly to a deterministic static fill.

## 5. Axis Positions

- **Theme**: **Light-first** (warm archival linen paper), paired with a calibrated matte warm-obsidian dark mode (`prefers-color-scheme` + explicit masthead toggle).
- **Tone**: Editorial & Technical — calm, precise, archival luxury.
- **Density**: Airy around the masthead and intake console; balanced-to-dense in the playlist catalog sheet so selecting 15 tracks out of 80 is effortless.
- **Shape**: Sharp-architectural (`2px`–`4px` crisp corners, `1px` warm stone borders, tonal surface recession rather than heavy drop shadows).
- **Typography**: Editorial serif display + industrial grotesk UI + precision monospace telemetry.

## 6. Design Tokens

### Typography (`next/font/google`)

| Role | Font Family | CSS Variable | Usage |
|---|---|---|---|
| Display / Wordmark / Titles | `Instrument Serif` | `--font-display` | `stash.` wordmark, single video title, playlist heading |
| UI & Body | `IBM Plex Sans` | `--font-sans` | Controls, playlist track titles, labels, helper copy |
| Telemetry & Specs | `IBM Plex Mono` | `--font-mono` | Track indices (`01`), durations (`04:12`), resolutions, codecs, bitrates, MB/s, ETA (`tabular-nums`) |

Type scale (strict token values):
- Display XL (Wordmark / Hero Title): `2.25rem` / `1.08` (`-0.015em`)
- Display MD (Media / Playlist Title): `1.75rem` / `1.15` (`-0.01em`)
- Heading SM (Section / Drawer): `1rem` / `1.35` (`500` weight)
- Body Base: `0.9375rem` (15px) / `1.5`
- Body SM: `0.8125rem` (13px) / `1.45`
- Mono Telemetry XS: `0.75rem` (12px) / `1.3` (`font-variant-numeric: tabular-nums`)
- Micro Label: `0.6875rem` (11px) / `1.2` (`uppercase`, `tracking-[0.08em]`)

### Color Palette (WCAG AA Verified)

#### Light Mode (Default — Warm Archival Linen)
- `--bg-canvas`: `#F6F4EF` (Page background)
- `--bg-surface`: `#FFFFFF` (Primary media deck, playlist sheet, drawer)
- `--bg-recessed`: `#ECE8DF` (Recessed control wells, table headers, subtle hover)
- `--ink-primary`: `#161513` (Primary ink — 16.4:1 vs canvas)
- `--ink-secondary`: `#57534C` (Secondary metadata — 6.9:1 vs white, 6.3:1 vs canvas)
- `--ink-muted`: `#6E6960` (Quiet hints — 5.1:1 vs white, 4.7:1 vs canvas)
- `--border-hairline`: `#DBD5C9` (Structural 1px dividers)
- `--border-strong`: `#B8B0A0` (Active input focus / hover borders)
- `--accent`: `#C43A27` (Braun Signal Vermilion — 5.3:1 with `#FFFFFF` text)
- `--accent-hover`: `#A82E1D`
- `--accent-tint`: `#FBECE9` (Subtle selected row / active capture wash)
- `--status-ready`: `#1E6B45` (Archival Forest)
- `--status-warn`: `#8F5600` (Ochre)
- `--status-error`: `#B42318` (Crimson)

#### Dark Mode (Matte Warm Obsidian — Never Cold Navy)
- `--bg-canvas`: `#131210`
- `--bg-surface`: `#1B1A17`
- `--bg-recessed`: `#24221E`
- `--ink-primary`: `#F2EFE9` (15.8:1 vs canvas)
- `--ink-secondary`: `#ADA79C` (7.4:1 vs surface)
- `--ink-muted`: `#8E887E` (4.9:1 vs surface)
- `--border-hairline`: `#2E2B26`
- `--border-strong`: `#47433B`
- `--accent`: `#E25440` (5.1:1 vs `#131210`; paired with `#131210` or `#FFFFFF` per AA contrast — white on `#C43A27` button fill in dark mode maintains 5.3:1!)
- `--accent-fill`: `#C43A27` (Button background in both themes guaranteeing ≥5.3:1 with white ink)
- `--accent-hover`: `#D94632`
- `--accent-tint`: `#2A1A17`
- `--status-ready`: `#36B374`
- `--status-warn`: `#E09F3E`
- `--status-error`: `#F06458`

### Spacing, Radius & Borders
- Spacing scale: `4px` base (`4, 8, 12, 16, 20, 24, 32, 40, 48, 64`).
- Radius scale: `--radius-sm: 2px`, `--radius-md: 4px`, `--radius-lg: 6px` (sharp architectural corners; pills only for 1px micro status dots).
- Borders: `1px solid var(--border-hairline)` defines hierarchy; minimal shadow (`0 1px 2px rgba(22, 21, 19, 0.04)`) only on floating drawers/menus.

### Layout Structure
Single-workspace **Command-Bar-Led Archival Studio** (`max-w-[1120px]` centered):
1. **Masthead (`56px`)**: `stash.` serif wordmark, live backend engine indicator (static icon + text + subtle status dot — never color-only), Light/Dark toggle, Transfers drawer trigger with active count.
2. **Intake Console**: Full-width hairline URL bar with paste detection, `VIDEO` / `PLAYLIST` badge, and submit CTA.
3. **Dynamic Stage**:
   - **Idle**: Quiet, generous negative space with keyboard shortcut reference (`⌘V` Paste, `Shift+Click` Range Select) and local session history (when present).
   - **Single Video**: Asymmetric 2-column split deck (Left: 16:9 media plate + mono duration/channel specs; Right: Video/Audio mechanical toggle, format presets with `REMUX · FAST` vs `TRANSCODE` tags, primary Download action, live Telemetry Ruler).
   - **Playlist**: Header metadata plate + Sticky selection action bar (Search filter, `Select All` / `None` / `Invert`, selected tally + total duration, Video/Audio + quality controls, primary `Stash N Tracks (.zip)` action) + Virtualized dense tracklist table with per-row checkboxes, `Shift+Click` range selection, inline live Telemetry Ruler, and per-track individual Download / Retry controls.
4. **Transfers Drawer**: Non-blocking slide-over sheet tracking active and completed jobs with cancel, retry, and re-download triggers.

### Motion Style
- Library: `motion/react` wrapped globally in `<MotionConfig reducedMotion="user">`.
- Single curve: `cubic-bezier(0.22, 1, 0.36, 1)`.
- Durations: `180ms` for segmented switch (`layoutId`) and hover states; `260ms` for stage/deck entrance; instant/static opacity crossfade under reduced motion.

## 7. Anti-Choices (Strictly Forbidden)

- No marketing landing-page skeleton (no Hero + 3 feature cards + Bento + logo strip + CTA footer).
- No `Inter`, `Geist`, or unstyled system sans.
- No purple/indigo/blue gradients, neon glows, or aurora backgrounds.
- No cold dark navy / pure `#000000` default theme.
- No glassmorphism / frosted blur cards or gradient borders.
- No puffy `rounded-2xl` bordered card grids with icon + title + text.
- No sparkle/gradient headline text, no emojis anywhere in UI, code, or logs.
- No fake metrics, placeholder download counts, or filler copy.
- No meaning conveyed exclusively by color or motion (always pair with static text/icon labels).

## 8. Accessibility Guardrails

- Visible `2px` focus rings (`outline-offset: 2px`) on every interactive element.
- Minimum `44×44px` tap targets on mobile viewports for all buttons, checkboxes, and switches.
- Semantic HTML landmarks (`<header>`, `<main>`, `<section>`), ordered headings, explicit `<label>`s for all inputs, meaningful `alt` attributes, full keyboard navigation.
- Complete **loading**, **empty**, and **error** states designed inline with every view.
