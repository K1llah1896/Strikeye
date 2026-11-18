# Design

## Context

See [proposal.md](./proposal.md) for motivation and scope. Currently:
- `internal/ui/theme/theme.go` exposes color tokens as package-level `var`s initialized once for a dark esports palette, and `app.go` wraps the root in `theme.EsportsTheme()`.
- `cmd/strikeye-agent/main.go` only emits `parse.started`, `parse.completed`, and `parse.failed` over IPC; `app.go` runs `NavigateTo` view loaders (`SelectDemo`, `LoadDemo`) synchronously on the UI thread.
- `internal/ui/views/analysis/analysis.go` steps 2D replay by `v.replayFrameIdx++` at most once per `Render` when `time.Since(v.lastStepAt) >= 31ms`, and `internal/ui/components/radar_2d_canvas.go` re-scales the full map PNG on every frame.
- `internal/ui/views/highlights/highlights.go` renders `Ticks: %d ~ %d` alongside tag `ui.Box` items without `Shrink(0)` inside a `420px` column, causing flex compression and multi-line wrapping.

## Goals / Non-Goals

**Goals:**
- Provide a persistent, live-switchable theme system (presets, custom accent color, custom background image + readability overlay) without breaking existing `theme.BgSurface` / `theme.AccentAmber` call sites across all 5 views.
- Ensure every long-running waiting scenario (demo parsing, Match Analysis loading, radar card / heatmap generation, Spectator/Highlights demo loading) renders a clear progress bar and never freezes the UI thread.
- Make 32Hz 2D tactical radar playback smooth and time-accurate with configurable playback speed (`0.5x`–`8x`).
- Keep Highlight Library clip cards clean (no tick range, single-line horizontally scrollable non-wrapping tags).

**Non-Goals:**
- Changing the underlying `demoinfocs` parser algorithm or `CS2RPL01` binary format.
- Adding a full CSS-style theme engine or per-widget arbitrary style overrides.
- Supporting animated GIF/video desktop wallpapers (static PNG/JPG background images only).

## Decisions

1. **Dynamic Theme Tokens via `theme.Apply(ThemeConfig)` + Background Layer in `App.Render`**
   - *Choice*: Keep existing exported `theme.BgBase`, `theme.BgSurface`, `theme.AccentAmber`, etc. as mutable palette tokens updated by `theme.Apply(cfg.Theme)` (plus a `theme.ActiveTheme()` builder for `ui.Themed`). Load and pre-scale/decode the custom background image into a cached `*ui.Bitmap` only when the path changes.
   - *Rationale*: Over 100 call sites across 5 views and components reference `theme.*` directly; mutating the active palette tokens via `Apply` makes every view react immediately without rewriting every component signature.
   - *Alternative considered*: Passing a `Theme` struct down to every `Render` method—rejected due to excessive churn with no functional gain in a single-window desktop app.

2. **Throttled `parse.progress` Events & Async `NavigateTo` Loading**
   - *Choice*: Add an optional `OnProgress func(stage string, progress float64)` callback to `parser.ParseOptions`. Report progress by byte-offset / parser progress during `ParseToEnd()` (throttled to ~100–200ms or 2% increments) and discrete stage milestones (DB persistence, replay cache, radar/highlights). Publish `parse.progress` over IPC to update `library.View` cards and `app.go` header.
   - *Choice for UI*: `NavigateTo` sets `activeView` immediately and launches a `go` routine for `SelectDemo` / `LoadDemo` with a thread-safe `loadingState` (`active bool, progress float64, stage string`), rendering `theme.ProgressBar` until completion.
   - *Rationale*: Keeps the MyGo UI thread unblocked at 60fps while giving immediate visual feedback.

3. **2D Radar Pre-scaled Backdrop Cache + Wall-Clock Delta Stepping**
   - *Choice*: In `Radar2DCanvas.ConfigureMap`, rasterize the background (decoded PNG scaled to `size x size`, or procedural grid) once into a cached `*image.RGBA`. In `RenderReplayFrame`, copy `bgCache.Pix` directly into the `gg` target buffer instead of re-scaling the PNG every frame.
   - *Choice for Playback*: Track fractional frame accumulator `replayFramePos float64` and `lastStepAt time.Time`. On each render tick while `replayPlaying`, advance `replayFramePos += elapsed.Seconds() * 32.0 * v.replaySpeed`, clamp/loop, and trigger continuous UI redraws (via MyGo invalidation/ticker) so playback never stalls waiting for mouse events.

4. **Highlight Card Layout Cleanup**
   - *Choice*: Remove `Ticks: %d ~ %d` from `renderClipCard` and place tags inside `ui.ScrollHorizontal(c).FillWidth()` with `.Shrink(0)` on each tag pill.
   - *Rationale*: Matches `theme.Capsule` behavior and prevents flex-shrink from wrapping CJK/emoji tag text.

## Risks / Trade-offs

- **Background image readability** → Mitigated by rendering a configurable translucent overlay (`overlay_alpha`, default `0.72`) over the background image and configurable card opacity (`card_opacity`, default `0.90`).
- **Concurrent UI state during async `SelectDemo`** → Mitigated by guarding `loading`, `progress`, and loaded view slices with a mutex (or atomic swap on completion) and ignoring stale completions if the user switches demos/views mid-load.
- **High-speed 2D radar CPU usage at 8x** → Caching the `560x560` RGBA backdrop removes the dominant per-frame cost (`gg.Scale` + `DrawImage`), keeping per-frame overlay drawing fast.
