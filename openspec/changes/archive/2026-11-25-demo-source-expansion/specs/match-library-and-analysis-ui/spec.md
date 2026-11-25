# Spec Delta

## Purpose

Keeps the Match Library platform filter in lockstep with the canonical demo source registry so every stored platform value has a matching option.

## ADDED Requirements

### Requirement: Platform filter mirrors canonical source registry
The Match Library SHALL build its platform filter options from the canonical source registry, always include an all-platforms option and an unknown-source option, and compare filter selections against stored values case-insensitively after normalization, so that legacy stored labels remain selectable.

#### Scenario: Every stored platform is filterable
- **WHEN** demos exist with `source_platform` values `faceit`, `esl`, `perfect_world`, and `5e`
- **THEN** the filter offers a matching option for each and selecting it returns exactly those demos

#### Scenario: Legacy label still matches
- **WHEN** a demo row stores the legacy value `perfectworld` and the user selects Perfect World
- **THEN** the demo is included in the result set

#### Scenario: Unknown source remains reachable
- **WHEN** the user selects the unknown-source option
- **THEN** demos stored with `unknown` are listed and demos with a recognized source are excluded
