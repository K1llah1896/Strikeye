# Design: Open Demo in CS2

## URL shape

`steam://rungameid/730//+playdemo%20<escaped path>` is the documented Community
pattern for launching a game with console arguments. The demo path is
percent-encoded with `url.PathEscape`; `+` is additionally escaped because Steam
treats it as an argument separator in `rungameid` URLs.

## Config

A tick seek is emitted as `demo_goto <tick>` in a cfg body. Nothing is written to
disk by Strikeye: the operator decides whether to place the cfg in the CS2 cfg
folder and exec it. Tick 0 produces no config.

## Path assumptions

`demo_path` is the path Strikeye stores. CS2 plays demos relative to its own
`csgo` directory, so the operator may need to copy the file and adjust the path;
the printed `file_name` is provided for that copy step.

## Verification

Unit tests cover escaping, single `+`, tick-gated config, and file-name
normalization; a real invocation against an imported demo prints the expected
JSON.
