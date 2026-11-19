# Spec Delta

## ADDED Requirements

### Requirement: Cinema-grade floating viewport and Spatial Glass Spectator controls
The `Spectator` view SHALL present its video player inside a borderless rounded cinema viewport above a low-chrome pill round selector and luminous event timeline, paired with a translucent Spatial Glass POV switcher panel for CT, T, and Observer perspectives.

#### Scenario: Viewing Spectator in cinema glass layout
- **WHEN** the user opens `Spectator` for a parsed match
- **THEN** the video viewport renders with rounded corners and no heavy outer border, flanked by the glass POV switcher and pill round track

### Requirement: Bento Highlight Library cards and dual-pane glass Montage Workbench
The `Highlight Library` tab SHALL render clip items as Bento glass cards with single-line translucent tag pills and embedded trim controls, and the `Montage Workbench` tab SHALL organize the sequence track and export configuration into a balanced dual-pane Spatial Glass layout.

#### Scenario: Browsing highlights and configuring a montage
- **WHEN** the user switches between `Highlight Library` and `Montage Workbench` via the segmented pill control
- **THEN** clip cards and montage panels render on matching Spatial Glass surfaces with clear visual hierarchy and non-wrapping tag pills
