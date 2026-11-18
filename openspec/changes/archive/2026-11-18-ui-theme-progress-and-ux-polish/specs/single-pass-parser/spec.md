# Spec Delta

## ADDED Requirements

### Requirement: Incremental parse progress reporting
The parser and agent parse worker SHALL report throttled stage and percentage progress (`0.0–1.0`) during demo scanning, highlight/metric computation, and SQLite persistence, publishing `parse.progress` IPC events with `demo_id`, `stage`, and `progress` to subscribed UI clients.

#### Scenario: Demo parsing emits progress updates
- **WHEN** `strikeye-agent` parses a pending `.dem` file
- **THEN** `parse.progress` events are emitted periodically with monotonically increasing progress values and descriptive stage labels before `parse.completed`
