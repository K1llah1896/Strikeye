# Spec Delta

## ADDED Requirements

### Requirement: Clean localized unrendered POV badge in Spectator
The `Spectator` unrendered POV summary card SHALL display `未生成独立视角视频` without raw English parenthetical suffixes such as `(unrendered)`.

#### Scenario: Viewing an unrendered POV in Spectator
- **WHEN** the user selects a player/round with no verified MP4 in `Spectator`
- **THEN** the status capsule reads `未生成独立视角视频` with no raw English enum suffix
