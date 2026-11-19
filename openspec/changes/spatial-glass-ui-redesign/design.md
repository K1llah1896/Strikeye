# Design

## Context

Strikeye's desktop UI is built on `github.com/egoist/mygo/ui` (native Skia + custom shader rendering, without WebView/HTML/CSS). As noted in `proposal.md`, the current UI looks dated because the default `Pure Black` preset disables gradients (`hasGradient == false`), leaving `GlassEffect()` inactive and causing every panel, button, and capsule to fall back to opaque dark boxes with uniform 1px solid borders (`#222228`).

`mygo/ui` already exposes `RadialGradient`, `LinearGradient`, `Blur`, `Shadow`, `Effect(shader)`, `Transition`, and `ui.Icon(*ui.SVG)`, and `internal/ui/visual_audit_test.go` provides offscreen PNG rendering verification.

## Goals / Non-Goals

**Goals:**
- Establish a unified **Spatial Glass + Bento 2.0** design system in `internal/ui/theme` with an ambient radial-glow backdrop, translucent frosted glass surfaces, luminous hairline borders, soft spatial shadows, and expanded typographic hierarchy (`28px–36px` `Rajdhani` KPIs vs `10px–11px` tracked labels).
- Introduce crisp inline SVG vector icons (`internal/ui/theme/icons.go`) for navigation, actions, status indicators, and empty states.
- Refactor the main shell (`internal/ui/app.go`), guard dialog (`internal/ui/guard/guard.go`), shared components (`internal/ui/components/`), and all 5 views (`library`, `spectator`, `analysis`, `highlights`, `settings`) to use the new primitives consistently.
- Preserve all existing functional behaviors, IPC contracts, localized tactical labels, and the `1360x840` unclipped `Settings & Tasks` render-queue visibility requirement.

**Non-Goals:**
- Introducing any WebView, HTML/CSS/JS runtime, or external icon font dependencies.
- Modifying SQLite schemas, `strikeye-agent` JSON-RPC methods, demo parsing, or FFmpeg composition logic.

## Decisions

### 1. Ambient Spatial Backdrop via `RadialGradient` + `Blur`
- **Choice**: Render an obsidian base (`#05070B`) layered with 2–3 large, soft `RadialGradient` orbs (`Blur`-softened cold cyan and indigo/emerald ambient glows) inside a non-interactive backdrop container (`IgnoreInput(true)`), and activate `GlassEffect()` across glass panels by default.
- **Rationale**: Unlike static raster wallpapers, procedural radial-glow layers scale cleanly to any window size with zero asset overhead and immediately give `frostedGlassShader` real tonal variation to refract.
- **Alternative Considered**: Shipping bundled PNG gradient wallpapers — rejected due to binary size bloat and scaling artifacts on ultrawide displays.

### 2. Rim-Light Glass Surfaces vs. Solid `1px` Borders
- **Choice**: Replace `#222228` solid borders on cards and containers with translucent surface fills (`GlassSurface(...)`), ultra-subtle `1px` luminous rim borders (`rgba(255, 255, 255, 0.08)`), larger radii (`18px–22px`), and soft `Shadow(...)` depth.
- **Rationale**: Separates foreground Bento modules from the ambient backdrop through depth and specular edge cues rather than harsh wireframe grids.

### 3. Inline SVG Vector Iconography (`internal/ui/theme/icons.go`)
- **Choice**: Define a curated set of minimalist `24x24` stroke-based `ui.SVG` icons rendered via `ui.Icon(c, svg)` and tinted dynamically with `TextPrimary`, `TextMuted`, or active accent colors.
- **Rationale**: Eliminates reliance on emoji glyphs or plain text bars, stays razor-sharp at any DPI, and requires no external font files.

### 4. Standardized Bento & Segmented Primitives
- **Choice**: Add `GlassCard`, `BentoCard`, `StatTile`, `SegmentedControl`, and `PillBadge` in `internal/ui/theme/theme.go`, and migrate all 5 views to compose from these shared primitives.
- **Rationale**: Prevents visual drift between views and keeps per-view code concise and maintainable.

### 5. Backward-Compatible Preset Upgrade
- **Choice**: Upgrade the default `Spatial Obsidian` / `Pure Black` preset tokens to enable Spatial Glass out of the box while keeping all existing preset IDs (`esports_dark`, `aurora_gradient`, `light_clean`, etc.) and custom background image support intact.
- **Rationale**: Existing user `config.json` files continue to load without migration errors.

## Risks / Trade-offs

- **[Risk] Shader & blur overdraw on low-end/integrated GPUs** → **Mitigation**: Keep the ambient backdrop as a flat non-interactive layer with a small number of large radial orbs rather than per-card dynamic blurs, and keep `frostedGlassShader` minimal.
- **[Risk] Vertical space pressure in `Settings & Tasks` at `1360x840`** → **Mitigation**: Use compact Bento group padding and multi-column grid arrangement for environment probes and appearance controls so `4. 后台渲染任务队列` remains unclipped.
- **[Risk] Visual regressions across 5 views** → **Mitigation**: Run `go test ./internal/ui/...` including `TestVisualAuditOffscreenRender` to inspect rendered offscreen PNGs for shell, library, analysis, and settings views.
