# Spec Delta

## Purpose

Defines background render task planning, per-demo batch scheduling, UI/CS2 mutual-exclusion enforcement, process ownership safety, CS2 user configuration backup/restore, and crash recovery.

## ADDED Requirements

### Requirement: Three-tier render policy task planning
The scheduler SHALL generate `render_tasks` after demo parsing according to `RenderConfig.Policy` (`smart_highlights`, `full_pov`, or `manual_only`), resolving the target player from `PrimarySteamID64` or falling back to the match MVP by Strikeye Rating.

#### Scenario: Smart highlights policy enqueues qualifying clips
- **WHEN** a demo finishes parsing with `policy` set to `smart_highlights`
- **THEN** the scheduler enqueues idempotent `render_tasks` (priority `50`) for the target player's `highlight_clips` meeting `HighlightImportanceMin`

#### Scenario: Full POV policy enqueues survived rounds
- **WHEN** a demo finishes parsing with `policy` set to `full_pov`
- **THEN** the scheduler enqueues idempotent `render_tasks` (priority `30`) for each round where the target player participated

#### Scenario: Manual-only policy creates no automatic tasks
- **WHEN** a demo finishes parsing with `policy` set to `manual_only`
- **THEN** the scheduler creates zero automatic `render_tasks` and waits for explicit manual requests (priority `100`)

### Requirement: Per-demo batch session grouping
The scheduler SHALL group all `queued` tasks sharing the same `demo_id` ordered by ascending `start_tick` into a single batch session so CS2 launches once per demo batch.

#### Scenario: Multiple queued tasks for one demo share one session
- **WHEN** three `queued` render tasks exist for the same `demo_id`
- **THEN** the scheduler executes all three tasks in ascending `start_tick` order within a single HLAE/CS2 session

### Requirement: Strict UI and CS2 mutual exclusion
The scheduler SHALL NOT launch HLAE/CS2 while `Local\StrikeyeUISingletonMutex` is held or within 15 seconds of UI release unless explicitly triggered via `render.start_now`, and SHALL support `render.abort_and_cleanup` to terminate rendering cleanly.

#### Scenario: Background render waits for UI cooldown
- **WHEN** `Local\StrikeyeUISingletonMutex` is held or released less than 15 seconds ago
- **THEN** the scheduler keeps queued tasks pending without acquiring `Local\StrikeyeCS2RenderActiveMutex`

#### Scenario: Immediate render and safe abort via IPC
- **WHEN** the agent receives `render.abort_and_cleanup` during an active render session
- **THEN** the scheduler terminates only Strikeye-owned CS2/HLAE processes, restores player configs, marks interrupted tasks `queued` or `cancelled`, and releases `Local\StrikeyeCS2RenderActiveMutex`

### Requirement: External CS2 deferral and process ownership guard
The process guard SHALL defer rendering while any non-Strikeye `cs2.exe` process is running and SHALL bind spawned HLAE/CS2 processes to a Windows Job Object (`KILL_ON_JOB_CLOSE`) and session token, never terminating unrelated `cs2.exe` instances.

#### Scenario: User-launched CS2 defers background render
- **WHEN** a `cs2.exe` process not tracked by Strikeye is detected before launch
- **THEN** the scheduler leaves tasks `queued` and emits a deferral status event without killing the user's CS2 process

#### Scenario: Controlled shutdown cleans only Strikeye process tree
- **WHEN** a render session finishes or aborts
- **THEN** the guard closes only the tracked Job Object / `(PID, CreationTime, SessionToken)` processes

### Requirement: Atomic CS2 user config snapshot and rollback
The config guard SHALL back up player `config.cfg`, `cs2_user_keys_*.vcfg`, `cs2_user_convars_*.vcfg`, and `cs2_video.txt` with `active_restore_manifest.json` before launching CS2 and atomically restore them on exit, abort, or agent startup.

#### Scenario: Configs restored after normal or aborted session
- **WHEN** a render session exits normally or is aborted
- **THEN** all backed-up player configuration files are restored from `active_restore_manifest.json` and the manifest is cleared

### Requirement: Agent startup crash recovery
Upon startup, `strikeye-agent` SHALL execute `RestoreIfPending()`, clean temporary `_strikeye_playback_*.dem` and incomplete capture directories, and reconcile orphaned `running` tasks in `render_tasks`.

#### Scenario: Orphaned running task recovered on restart
- **WHEN** `strikeye-agent` starts and finds a `render_tasks` row still in `running` state
- **THEN** it restores pending CFG backups, cleans temporary files, and resets the task to `queued` (if `retry_count < max_retries`) or marks it `failed` with `error_code = 'AGENT_INTERRUPTED'`
