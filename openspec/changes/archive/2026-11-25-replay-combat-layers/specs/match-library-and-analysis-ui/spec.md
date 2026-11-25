# Spec Delta

## Purpose

让 2D 回放展示并可切换交火图层。

## ADDED Requirements

### Requirement: 2D replay combat layers

The `2D Replay` tab SHALL render shot tracers, death markers, and C4 plant/explosion overlays from the round's combat events.

#### Scenario: Shot tracers fade over time
- **WHEN** a frame is past a shot's tick by less than the fade window
- **THEN** a line from the shooter to the aim endpoint is drawn with decreasing opacity

#### Scenario: Death markers persist for the round
- **WHEN** a player dies in the selected round
- **THEN** a marker remains at the death position, with a killer-to-victim line during the recent window

#### Scenario: C4 plant and blast radius are visible
- **WHEN** the round has a plant event and playback reaches it
- **THEN** a C4 marker appears at the plant position, and after detonation a 500-unit blast circle is drawn

#### Scenario: Layers can be toggled independently
- **WHEN** the user unchecks a layer toggle
- **THEN** only that layer disappears while the remaining layers and player poses keep rendering

#### Scenario: Legacy caches render without combat layers
- **WHEN** the round comes from a v1 replay cache
- **THEN** the canvas renders poses and utility effects normally and shows no combat layers
