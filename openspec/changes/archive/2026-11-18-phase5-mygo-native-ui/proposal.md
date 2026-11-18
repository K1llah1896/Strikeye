# Proposal: Phase 5 — MyGo Native UI Desktop Client

## Why

Phases 1–4 delivered Strikeye's pure-Go backend (`strikeye-agent`, SQLite WAL storage, Single-Pass demo parser, 32Hz `CS2RPL01` replay binary, CS Data Radar generator, FFmpeg composer, and HLAE+CS2 background recorder), while `cmd/strikeye` remains a placeholder skeleton. Users need the standalone, zero-WebView MyGo Native UI desktop client (`strikeye.exe`) so they can browse matches, watch verified multi-POV videos with synchronized event timelines and key HUDs, inspect 2D tactical replays and KDE heatmaps, generate radar cards, and compose/export highlight montages without launching CS2 or any browser runtime.

## What Changes

- Add `github.com/egoist/mygo` (`ui` package) as a pure-Go, no-CGO dependency for GPU-rendered native desktop windows (Direct3D 11 on Windows, OpenGL on Linux, Metal on macOS).
- Implement `cmd/strikeye` startup lifecycle: probe `Local\StrikeyeCS2RenderActiveMutex` to show the lightweight Render Guard Dialog when CS2 rendering is active, acquire `Local\StrikeyeUISingletonMutex`, auto-launch `strikeye-agent` if not running, and initialize the main MyGo Native UI shell.
- Build the dark esports theme (`#0F141C` background, `#171E29` cards, `#F59E0B` / `#38BDF8` accents) with embedded `assets/fonts/` and `assets/maps/` (radar PNGs + coordinate metadata with procedural grid fallback).
- Implement the 4 core native UI components under `internal/ui/components/`:
  - Embedded FFmpeg-decoded video & audio player (`video_player.go`) with Event Timeline anchors and dual-slider `In/Out` trimming.
  - Tick-synchronized Key HUD widget (`key_hud_widget.go`) driven by `CS2RPL01` / `m_nButtons`.
  - 32Hz 2D tactical replay & KDE heatmap canvas (`radar_2d_canvas.go`).
  - CS Data Radar preview & workbench widget (`radar_hex_preview.go`).
- Implement the 5 primary workspace views under `internal/ui/views/`: `library`, `spectator`, `analysis`, `highlights`, and `settings`.
- Extend `cmd/strikeye-agent` IPC handlers with `montage.export`, `render.cancel`, `render.retry`, and `config.reload` to support interactive UI actions and progress events.

## Capabilities

### New Capabilities
- `native-ui-shell-and-guard`: Covers `strikeye.exe` startup mutex probing, the Render Guard Dialog (`继续后台渲染` / `安全中止渲染并打开 Strikeye`), `strikeye-agent` auto-launch, MyGo Native UI window shell, dark esports theme, IPC event subscription, `Settings & Tasks` view, and agent IPC extensions (`montage.export`, `render.cancel`, `render.retry`, `config.reload`).
- `match-library-and-analysis-ui`: Covers the `Match Library` view (search, filters, status capsules, favorites/notes, manual import, watch-folder config), `assets/maps/` radar map assets, and the 5-tab `Match Analysis` view (Scoreboard, Rounds & Economy, Players & CS Data Radar workbench, 32Hz 2D Replay canvas, and KDE Heatmaps).
- `spectator-and-highlights-ui`: Covers the `Spectator` multi-POV video player (strict real-MP4 vs unrendered empty-state separation, 2.0s-pre-event timeline anchors, tick-synced Key HUD, embedded FFmpeg playback) and `Highlights & Montage` workbench (highlight filtering, `In/Out` trim + re-render, montage ordering, radar intro insertion, and async FFmpeg export).

### Modified Capabilities
<!-- None: existing ipc-and-mutex and render-scheduler-and-safety requirements already define the underlying mutexes and render.start_now / render.abort_and_cleanup RPCs. -->

## Impact

- **Dependencies**: `go.mod` / `go.sum` gains `github.com/egoist/mygo`.
- **Binaries**: `cmd/strikeye/main.go` becomes the full MyGo Native UI desktop entry point; `cmd/strikeye-agent/main.go` gains montage export, task cancel/retry, and config reload IPC handlers.
- **New Packages & Assets**: `internal/ui/` (`app.go`, `theme/`, `guard/`, `components/`, `views/{library,spectator,analysis,highlights,settings}`) and `assets/maps/`.
- **Tech Stack Purity**: Zero WebView2, HTML, CSS, JS, React, Python, or OBS dependencies.
