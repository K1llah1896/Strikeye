package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"strikeye/internal/config"
	"strikeye/internal/export"
	"strikeye/internal/ipc"
	"strikeye/internal/mapassets"
	"strikeye/internal/mapgeom"
	"strikeye/internal/parser"
	"strikeye/internal/recorder"
	"strikeye/internal/storage"
	"strikeye/internal/toolbundle"
	"strikeye/internal/version"
)

// Run executes one CLI command. `args` are the arguments after the program name,
// exactly as `os.Args[1:]` would hold them; the merged `strikeye` binary passes
// its own tail here after stripping the `cli` selector.
//
// Run terminates the process for error paths the CLI reports through exit codes,
// which is what a command-line entry point is expected to do.
func Run(args []string) {
	fs := flag.NewFlagSet("strikeye", flag.ExitOnError)
	showVersion := fs.Bool("version", false, "Print version information and exit")
	envCheck := fs.Bool("env-check", false, "Print runtime environment summary and exit")
	_ = fs.Parse(args)

	if *showVersion {
		fmt.Println(version.Summary("strikeye"))
		return
	}
	if *envCheck || (fs.NArg() > 0 && fs.Arg(0) == "env-check") {
		fmt.Printf("%s: ready (phase2)\n", version.Summary("strikeye"))
		return
	}

	if fs.NArg() > 0 {
		// subArgs holds the command's own flags; the per-command FlagSet shadows
		// the outer variable, so the slice is taken once, up front.
		subArgs := fs.Args()[1:]
		switch fs.Arg(0) {
		case "doctor":
			fs := flag.NewFlagSet("doctor", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := runDoctor(ctx, dataRoot, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "doctor failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "parse":
			fs := flag.NewFlagSet("parse", flag.ExitOnError)
			fileFlag := fs.String("file", "", "Path to CS2 .dem file")
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			forceFlag := fs.Bool("force", false, "Force re-parse even if already cached")
			_ = fs.Parse(subArgs)

			if *fileFlag == "" {
				fmt.Fprintln(os.Stderr, "parse failed: --file is required")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := runParse(ctx, dataRoot, *fileFlag, *forceFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "parse failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "inspect":
			fs := flag.NewFlagSet("inspect", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Inspect persisted match by demo ID")
			rplFlag := fs.String("rpl", "", "Inspect CS2RPL01 binary replay file")
			roundFlag := fs.Uint("round", 0, "Optional round number when inspecting --rpl")
			_ = fs.Parse(subArgs)

			if *demoIDFlag <= 0 && *rplFlag == "" {
				fmt.Fprintln(os.Stderr, "inspect failed: specify --demo-id or --rpl")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := runInspect(ctx, dataRoot, *demoIDFlag, *rplFlag, uint32(*roundFlag), os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "inspect failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "export":
			fs := flag.NewFlagSet("export", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Export one parsed match by demo ID")
			steamIDFlag := fs.String("steamid", "", "Export cross-match aggregate for one SteamID64")
			allFlag := fs.Bool("all", false, "Export every parsed match, one directory per match")
			teamFlag := fs.String("team", "", "Batch export: only matches featuring this team")
			mapFlag := fs.String("map", "", "Batch export: only matches on this map")
			limitFlag := fs.Int("limit", 0, "Batch export: stop after N matches")
			outFlag := fs.String("out", "", "Output directory (default <DataRoot>/media/exports)")
			formatFlag := fs.String("format", "both", "csv, json, html, xlsx, or both")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			filter := export.BatchFilter{Team: *teamFlag, Map: *mapFlag, Limit: *limitFlag}
			if err := runExportBatch(ctx, dataRoot, *demoIDFlag, *steamIDFlag, *outFlag, *formatFlag, filter, *allFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "export failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "profile":
			fs := flag.NewFlagSet("profile", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			steamIDFlag := fs.String("steamid", "", "SteamID64 to inspect or annotate")
			nameFlag := fs.String("name", "", "Set the display-name override")
			noteFlag := fs.String("note", "", "Set a free-form scouting note")
			tagsFlag := fs.String("tags", "", "Comma-separated tags replacing the current list")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := runProfile(ctx, dataRoot, *steamIDFlag, *nameFlag, *noteFlag, *tagsFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "profile failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "series":
			fs := flag.NewFlagSet("series", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			minMapsFlag := fs.Int("min-maps", 2, "Only list series with at least this many maps")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := runSeries(ctx, dataRoot, *minMapsFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "series failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "maps":
			fs := flag.NewFlagSet("maps", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			_ = fs.Parse(subArgs)
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runMapsList(dataRoot, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "maps failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "maps-fetch":
			fs := flag.NewFlagSet("maps-fetch", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			baseURLFlag := fs.String("base-url", "", "Override geometry dataset base URL")
			allFlag := fs.Bool("all", false, "Fetch every map in the manifest")
			var maps mapListFlag
			fs.Var(&maps, "map", "Map name to fetch (repeatable)")
			_ = fs.Parse(subArgs)
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runMapsFetch(dataRoot, *baseURLFlag, append(maps, fs.Args()...), *allFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "maps fetch failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "assets-pack":
			fs := flag.NewFlagSet("assets-pack", flag.ExitOnError)
			assetDirFlag := fs.String("asset-dir", "", "Directory for the embedded nav/callout blobs (default internal/mapassets/assets)")
			geometryOutFlag := fs.String("geometry-out", "", "Directory for the map_geometry.zip sidecar (default assets)")
			toolsOutFlag := fs.String("tools-out", "", "Directory for the bundled FFmpeg/HLAE archives (default assets)")
			skipNavFlag := fs.Bool("skip-nav", false, "Skip the embedded nav mesh and callout blobs")
			skipGeomFlag := fs.Bool("skip-geom", false, "Skip the collision geometry bundle")
			skipToolsFlag := fs.Bool("skip-tools", false, "Skip the bundled FFmpeg/HLAE archives")
			allGeomFlag := fs.Bool("all-geometry", false, "Bundle every map's geometry instead of the competitive pool")
			baseURLFlag := fs.String("geometry-base-url", "", "Override geometry dataset base URL")
			toolsAPIFlag := fs.String("tools-api-url", "", "Override the tool release API used for the bundled archives")
			_ = fs.Parse(subArgs)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := runAssetsPack(ctx, assetsPackOptions{
				AssetDir:    *assetDirFlag,
				GeometryOut: *geometryOutFlag,
				ToolsOut:    *toolsOutFlag,
				SkipNav:     *skipNavFlag,
				SkipGeom:    *skipGeomFlag,
				SkipTools:   *skipToolsFlag,
				AllGeometry: *allGeomFlag,
				BaseURL:     *baseURLFlag,
				ToolsAPI:    *toolsAPIFlag,
			}, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "assets-pack failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "navs":
			fs := flag.NewFlagSet("navs", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			_ = fs.Parse(subArgs)
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runNavsList(dataRoot, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "navs failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "navs-fetch":
			fs := flag.NewFlagSet("navs-fetch", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			baseURLFlag := fs.String("base-url", "", "Override awpy-data release base URL")
			allFlag := fs.Bool("all", false, "Fetch navigation meshes for every map in the release")
			var maps mapListFlag
			fs.Var(&maps, "map", "Map name to fetch (repeatable)")
			_ = fs.Parse(subArgs)
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runNavsFetch(dataRoot, *baseURLFlag, append(maps, fs.Args()...), *allFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "navs-fetch failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "callouts-fetch":
			fs := flag.NewFlagSet("callouts-fetch", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			_ = fs.Parse(subArgs)
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runCalloutsFetch(dataRoot, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "callouts-fetch failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "area":
			fs := flag.NewFlagSet("area", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			mapFlag := fs.String("map", "", "Map name (e.g. de_dust2)")
			xFlag := fs.Float64("x", 0, "World X")
			yFlag := fs.Float64("y", 0, "World Y")
			zFlag := fs.Float64("z", 0, "World Z")
			_ = fs.Parse(subArgs)
			if *mapFlag == "" {
				fmt.Fprintln(os.Stderr, "area failed: --map is required")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			point := mapgeom.Vec3{float32(*xFlag), float32(*yFlag), float32(*zFlag)}
			if err := runArea(dataRoot, *mapFlag, point, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "area failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "navpath":
			fs := flag.NewFlagSet("navpath", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			mapFlag := fs.String("map", "", "Map name (e.g. de_dust2)")
			fromFlag := fs.String("from", "", "Start callout ID or label")
			toFlag := fs.String("to", "", "End callout ID or label")
			_ = fs.Parse(subArgs)
			if *mapFlag == "" || *fromFlag == "" || *toFlag == "" {
				fmt.Fprintln(os.Stderr, "navpath failed: --map, --from and --to are required")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runNavPath(dataRoot, *mapFlag, *fromFlag, *toFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "navpath failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "play":
			fs := flag.NewFlagSet("play", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID to prepare for CS2 playback")
			tickFlag := fs.Int64("tick", 0, "Optional start tick for the generated cfg")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := runPlay(ctx, dataRoot, *demoIDFlag, *tickFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "play failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "sharecode":
			fs := flag.NewFlagSet("sharecode", flag.ExitOnError)
			codeFlag := fs.String("code", "", "Match share code (CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx)")
			_ = fs.Parse(subArgs)
			if err := runSharecode(*codeFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "sharecode failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "mapcontrol":
			fs := flag.NewFlagSet("mapcontrol", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			stepFlag := fs.Float64("step", 5, "Sampling interval in seconds")
			radiusFlag := fs.Float64("radius", 1100, "Control reach radius in world units")
			effectsFlag := fs.Bool("effects", true, "Discount areas hidden by smoke or cut by fire")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runMapControl(context.Background(), dataRoot, *demoIDFlag, *stepFlag, *radiusFlag, *effectsFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "mapcontrol failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "sql":
			fs := flag.NewFlagSet("sql", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			queryFlag := fs.String("query", "", "Read-only SQL query")
			formatFlag := fs.String("format", "json", "json or csv")
			limitFlag := fs.Int("limit", 200, "Maximum rows to return (0 = no cap)")
			_ = fs.Parse(subArgs)

			if *queryFlag == "" {
				fmt.Fprintln(os.Stderr, "sql: --query is required")
				os.Exit(2)
			}
			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runSQL(context.Background(), dataRoot, *queryFlag, *formatFlag, *limitFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "sql failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "killctx":
			fs := flag.NewFlagSet("killctx", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			storeFlag := fs.Bool("store", false, "Persist the annotations into the kill_context table")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runKillContext(context.Background(), dataRoot, *demoIDFlag, *storeFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "killctx failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "zones":
			fs := flag.NewFlagSet("zones", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runZones(context.Background(), dataRoot, *demoIDFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "zones failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "nades":
			fs := flag.NewFlagSet("nades", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			minThrowsFlag := fs.Int("min-throws", 1, "Only report line-ups thrown at least this many times")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runNades(context.Background(), dataRoot, *demoIDFlag, *minThrowsFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "nades failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "gsi":
			fs := flag.NewFlagSet("gsi", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			listenFlag := fs.String("listen", "127.0.0.1:32123", "Address to listen on")
			tokenFlag := fs.String("token", "", "Shared token required in the Authorization header")
			configFlag := fs.String("config-out", "", "Where to write gamestate_integration cfg")
			latestFlag := fs.String("latest", "", "Where to mirror the latest state JSON")
			onceFlag := fs.Bool("once", false, "Exit after the first payload")
			timeoutFlag := fs.Duration("timeout", 30*time.Second, "How long --once waits for a payload")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := signalContext()
			defer cancel()
			err := runGSI(ctx, dataRoot, gsiOptions{
				listen:     *listenFlag,
				token:      *tokenFlag,
				configOut:  *configFlag,
				latestPath: *latestFlag,
				once:       *onceFlag,
				timeout:    *timeoutFlag,
			}, os.Stdout)
			if err != nil {
				fmt.Fprintf(os.Stderr, "gsi failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "tendencies":
			fs := flag.NewFlagSet("tendencies", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			teamFlag := fs.String("team", "", "Team name (as stored in demos)")
			topFlag := fs.Int("top", 8, "How many opening callouts to report")
			formatFlag := fs.String("format", "json", "Output format: json or csv")
			outFlag := fs.String("out", "", "Write to this file instead of stdout")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			rest := []string{"-team", *teamFlag, "-top", strconv.Itoa(*topFlag), "-format", *formatFlag}
			if *outFlag != "" {
				rest = append(rest, "-out", *outFlag)
			}
			if err := runTendencies(context.Background(), dataRoot, rest, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "tendencies failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "h2h":
			fs := flag.NewFlagSet("h2h", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			teamAFlag := fs.String("team-a", "", "First team")
			teamBFlag := fs.String("team-b", "", "Second team")
			formatFlag := fs.String("format", "json", "Output format: json or csv")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			rest := []string{"-team-a", *teamAFlag, "-team-b", *teamBFlag, "-format", *formatFlag}
			if err := runHeadToHead(context.Background(), dataRoot, rest, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "h2h failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "trends":
			fs := flag.NewFlagSet("trends", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			steamIDFlag := fs.String("steamid", "", "SteamID64")
			formatFlag := fs.String("format", "json", "Output format: json or csv")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			rest := []string{"-steamid", *steamIDFlag, "-format", *formatFlag}
			if err := runTrends(context.Background(), dataRoot, rest, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "trends failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "corpus":
			fs := flag.NewFlagSet("corpus", flag.ExitOnError)
			dirFlag := fs.String("dir", "tests", "Directory holding fixture demos")
			goldenFlag := fs.String("golden", filepath.Join("tests", "fixtures", "golden"), "Golden snapshot directory")
			updateFlag := fs.Bool("update", false, "Rewrite golden snapshots from the current parse")
			demoFlag := fs.String("demo", "", "Only process this fixture filename")
			workFlag := fs.String("work-dir", "", "Reusable scratch data root")
			geomFlag := fs.String("geometry-dir", "", "Directory of .tri map geometry for TTD metrics")
			_ = fs.Parse(subArgs)

			args := []string{"-dir", *dirFlag, "-golden", *goldenFlag, "-demo", *demoFlag}
			if *updateFlag {
				args = append(args, "-update")
			}
			if *workFlag != "" {
				args = append(args, "-work-dir", *workFlag)
			}
			if *geomFlag != "" {
				args = append(args, "-geometry-dir", *geomFlag)
			}
			if err := runCorpus(context.Background(), args, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "corpus failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "mixaudio":
			fs := flag.NewFlagSet("mixaudio", flag.ExitOnError)
			videoFlag := fs.String("video", "", "Input video clip")
			trackFlag := &stringListFlag{}
			fs.Var(trackFlag, "track", "Audio track spec path[@offset[:volume]] (repeatable)")
			outFlag := fs.String("out", "", "Output path (default <video>_mixed.mp4)")
			volFlag := fs.Float64("volume", 1.0, "Volume applied to the clip's own audio")
			audioOnlyFlag := fs.Bool("audio-only", false, "Also export the mixed audio as .m4a")
			ffmpegFlag := fs.String("ffmpeg", "", "ffmpeg executable")
			_ = fs.Parse(subArgs)

			if err := runMixAudio(context.Background(), mixaudioOptions{
				videoPath:  *videoFlag,
				trackSpecs: *trackFlag,
				outPath:    *outFlag,
				baseVolume: *volFlag,
				audioOnly:  *audioOnlyFlag,
				ffmpegExe:  *ffmpegFlag,
			}, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "mixaudio failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "search":
			fs := flag.NewFlagSet("search", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			queryFlag := fs.String("query", "", "Search text (players / teams / map / notes)")
			limitFlag := fs.Int("limit", 20, "Maximum hits")
			reindexFlag := fs.Bool("reindex", false, "Force a full index rebuild")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			args := []string{"-query", *queryFlag, "-limit", strconv.Itoa(*limitFlag)}
			if *reindexFlag {
				args = append(args, "-reindex")
			}
			if err := runSearch(context.Background(), dataRoot, args, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "search failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "strokes":
			fs := flag.NewFlagSet("strokes", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo id")
			roundFlag := fs.Int("round", -1, "Only strokes of this round (0 = match-wide)")
			clearFlag := fs.Bool("clear", false, "Delete matching strokes")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			rest := []string{"-demo-id", strconv.FormatInt(*demoIDFlag, 10), "-round", strconv.Itoa(*roundFlag)}
			if *clearFlag {
				rest = append(rest, "-clear")
			}
			if err := runStrokes(context.Background(), dataRoot, rest, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "strokes failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "avatars":
			fs := flag.NewFlagSet("avatars", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			steamIDFlag := fs.String("steamid", "", "Comma separated SteamID64 list")
			demoIDFlag := fs.Int64("demo-id", 0, "Fetch avatars for every player of this demo")
			outDirFlag := fs.String("out-dir", "", "Override the cache directory")
			refreshFlag := fs.Bool("refresh", false, "Ignore cached avatars")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			rest := []string{"-steamid", *steamIDFlag, "-demo-id", strconv.FormatInt(*demoIDFlag, 10)}
			if *outDirFlag != "" {
				rest = append(rest, "-out-dir", *outDirFlag)
			}
			if *refreshFlag {
				rest = append(rest, "-refresh")
			}
			if err := runAvatars(context.Background(), dataRoot, rest, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "avatars failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "buys":
			fs := flag.NewFlagSet("buys", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			roundFlag := fs.Int("round", 0, "Only this round")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runBuys(context.Background(), dataRoot, *demoIDFlag, *roundFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "buys failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "viewprefs":
			fs := flag.NewFlagSet("viewprefs", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			mapFlag := fs.String("map", "", "Map name to write (with --set)")
			xFlag := fs.Float64("x", 0, "Lower-layer horizontal offset in radar pixels")
			yFlag := fs.Float64("y", 0, "Lower-layer vertical offset in radar pixels")
			opacityFlag := fs.Float64("opacity", 1, "Lower-layer opacity (0.1-1.0)")
			setFlag := fs.Bool("set", false, "Persist the given values instead of listing")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			db, err := storage.OpenDefault(dataRoot)
			if err != nil {
				fmt.Fprintf(os.Stderr, "open storage: %v\n", err)
				os.Exit(1)
			}
			defer db.Close()
			if err := runViewPrefs(context.Background(), db, viewPrefsOptions{
				mapName: *mapFlag,
				offsetX: *xFlag,
				offsetY: *yFlag,
				opacity: *opacityFlag,
				set:     *setFlag,
			}, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "viewprefs failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "notes":
			fs := flag.NewFlagSet("notes", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			roundFlag := fs.Int("round", 0, "Round number (0 = whole match)")
			calloutFlag := fs.String("callout", "", "Callout id to pin")
			noteFlag := fs.String("note", "", "Note text")
			colorFlag := fs.String("color", "amber", "Marker color: amber|red|green|blue")
			deleteFlag := fs.Int64("delete", 0, "Delete annotation by id")
			clearFlag := fs.Bool("clear", false, "Delete every annotation of the demo")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runNotes(context.Background(), dataRoot, *demoIDFlag, *roundFlag, *calloutFlag, *noteFlag, *colorFlag, *deleteFlag, *clearFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "notes failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "download":
			fs := flag.NewFlagSet("download", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			cookieFlag := fs.String("cookies", "", "Cookie header value or path to a cookie file")
			tabFlag := fs.String("tab", "matchhistorypremier", "GCPD tab (comma separated for several)")
			limitFlag := fs.Int("limit", 0, "Max demos per tab (0 = all)")
			outFlag := fs.String("out", "", "Output dir (default: <DataRoot>/media/imports)")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			tabs := strings.Split(*tabFlag, ",")
			if err := runDownload(context.Background(), dataRoot, *cookieFlag, tabs, *limitFlag, *outFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "bans":
			fs := flag.NewFlagSet("bans", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID whose players should be checked (default: first parsed demo)")
			steamIDFlag := fs.String("steamid", "", "Comma separated SteamID64 list (overrides --demo-id)")
			refreshFlag := fs.Bool("refresh", false, "Ignore cached snapshots and ask Steam again")
			apiKeyFlag := fs.String("api-key", "", "Steam Web API key (default: $STRIKEYE_STEAM_API_KEY)")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ids := strings.Split(*steamIDFlag, ",")
			if err := runBans(context.Background(), dataRoot, *demoIDFlag, ids, *refreshFlag, steamAPIKey(*apiKeyFlag), os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "bans failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "voice-decode":
			fs := flag.NewFlagSet("voice-decode", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID to decode")
			toolFlag := fs.String("tool", "", "Path to an external Steam voice decoder")
			xuidFlag := fs.Uint64("xuid", 0, "Only export this speaker")
			outFlag := fs.String("out", "", "Output dir (default: <DataRoot>/media/voice/demo_<id>)")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			demoID := *demoIDFlag
			if demoID == 0 {
				db, err := storage.OpenDefault(dataRoot)
				if err != nil {
					fmt.Fprintf(os.Stderr, "voice-decode failed: %v\n", err)
					os.Exit(1)
				}
				demo, err := pickDemo(context.Background(), db, 0)
				db.Close()
				if err != nil {
					fmt.Fprintf(os.Stderr, "voice-decode failed: %v\n", err)
					os.Exit(1)
				}
				demoID = demo.ID
			}
			if err := runVoiceDecodeFiltered(context.Background(), dataRoot, demoID, *toolFlag, *outFlag, *xuidFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "voice-decode failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "voice-activity":
			fs := flag.NewFlagSet("voice-activity", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			roundFlag := fs.Int("round", 0, "Only report this round")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runVoiceActivity(context.Background(), dataRoot, *demoIDFlag, *roundFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "voice-activity failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "voice":
			fs := flag.NewFlagSet("voice", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Demo ID (default: first parsed demo)")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			if err := runVoice(context.Background(), dataRoot, *demoIDFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "voice failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "team":
			fs := flag.NewFlagSet("team", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			nameFlag := fs.String("name", "", "Team name as recorded on demos (case-insensitive)")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := runTeam(ctx, dataRoot, *nameFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "team failed: %v\n", err)
				os.Exit(1)
			}
			return

		case "render":
			fs := flag.NewFlagSet("render", flag.ExitOnError)
			dataRootFlag := fs.String("data-root", "", "Override Strikeye DataRoot directory")
			demoIDFlag := fs.Int64("demo-id", 0, "Optional demo ID for dry-run script generation")
			dryRunFlag := fs.Bool("dry-run", true, "Print render self-check and generated HLAE/CFG dry-run output")
			_ = fs.Parse(subArgs)

			dataRoot := resolveDataRootOrExit(*dataRootFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := runRenderDryRun(ctx, dataRoot, *demoIDFlag, *dryRunFlag, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "render failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Fprintln(os.Stderr, "usage: strikeye-cli [--version | --env-check | doctor | parse --file <dem> | inspect (--demo-id <id> | --rpl <path>) | export (--demo-id <id> | --steamid <id64>) [--format csv|json|xlsx|html|both|all] | profile --steamid <id64> | series | team --name <team> | sharecode --code <code> | play --demo-id <id> | maps | maps-fetch --map <map>|--all | navs | navs-fetch --map <map>|--all | assets-pack [--asset-dir <dir>] [--geometry-out <dir>] [--tools-out <dir>] [--skip-nav] [--skip-geom] [--skip-tools] | callouts-fetch | area --map <map> --x <x> --y <y> --z <z> | navpath --map <map> --from <place> --to <place> | sql --query <sql> [--format csv|json] [--limit <n>] | killctx [--demo-id <id>] [--store] | zones [--demo-id <id>] | mapcontrol [--demo-id <id>] [--step <sec>] [--effects=false] | nades [--demo-id <id>] [--min-throws <n>] | voice [--demo-id <id>] | bans [--demo-id <id> | --steamid <id64,...>] [--refresh] | download --cookies <file|header> [--tab <tab>] [--limit <n>] | voice-decode --demo-id <id> [--tool <path> | --xuid <id64>] | voice-activity --demo-id <id> [--round <n>] | notes [--demo-id <id>] [--round <n> --callout <id> --note <text>] | buys [--demo-id <id>] [--round <n>] | tendencies --team <name> [--top <n>] [--format json|csv] [--out <path>] | h2h --team-a <a> --team-b <b> [--format json|csv] | avatars (--steamid <id64,...> | --demo-id <id>) [--refresh] | search --query <text> [--limit <n>] [--reindex] | viewprefs [--set --map <map> --x --y --opacity] ... | mixaudio --video <clip> --track <path[@offset[:vol]]> [--audio-only] | strokes --demo-id <id> [--round <n>] [--clear] | trends --steamid <id64> [--format json|csv] | corpus [--update] [--geometry-dir <dir>] | gsi [--listen addr]")
	os.Exit(2)
}

func resolveDataRootOrExit(override string) string {
	if override != "" {
		installBundledMapAssets(override)
		return override
	}
	dataRoot, err := config.DefaultDataRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve data-root failed: %v\n", err)
		os.Exit(1)
	}
	installBundledMapAssets(dataRoot)
	return dataRoot
}

// installBundledMapAssets unpacks the map datasets shipped inside the binary so
// navmesh, callout and collision-geometry features work on a fresh checkout
// without any download step. Failures are reported but never fatal: a missing
// bundle only degrades the geometry-dependent metrics.
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func installBundledMapAssets(dataRoot string) {
	rep, err := mapassets.Install(dataRoot, mapassets.ExeDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: install bundled map assets: %v\n", err)
		return
	}
	if rep.NavsWritten > 0 || rep.CalloutsWritten {
		fmt.Fprintf(os.Stderr, "installed bundled map assets: %d nav meshes, callouts=%d\n", rep.NavsWritten, boolToInt(rep.CalloutsWritten))
	}
	provisionBundledTools(dataRoot)
}

// provisionBundledTools stages the FFmpeg/HLAE archives that ship beside the
// executables, so rendering and montage export work without visiting settings.
func provisionBundledTools(dataRoot string) {
	rep, err := toolbundle.Provision(dataRoot, mapassets.ExeDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: provision bundled tools: %v\n", err)
		return
	}
	if len(rep.Installed) > 0 {
		fmt.Fprintf(os.Stderr, "installed bundled tools: %s\n", strings.Join(rep.Installed, ", "))
	}
}

type parseCLISummary struct {
	DemoID              int64   `json:"demo_id"`
	MapName             *string `json:"map_name,omitempty"`
	TeamAName           string  `json:"team_a_name"`
	TeamBName           string  `json:"team_b_name"`
	TeamAScore          int     `json:"team_a_score"`
	TeamBScore          int     `json:"team_b_score"`
	TickRate            float64 `json:"tick_rate"`
	HalftimeRound       *int    `json:"halftime_round,omitempty"`
	RoundCount          int     `json:"round_count"`
	PlayerCount         int     `json:"player_count"`
	EventCount          int     `json:"event_count"`
	HighlightCount      int     `json:"highlight_count"`
	InputDataCapability string  `json:"input_data_capability"`
	ReplayCachePath     string  `json:"replay_cache_path"`
	ReusedCache         bool    `json:"reused_cache"`
	ElapsedMs           int64   `json:"elapsed_ms"`
}

func runParse(ctx context.Context, dataRoot, filePath string, force bool, out io.Writer) error {
	start := time.Now()
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	sum, err := parser.ParseAndPersist(ctx, db, parser.ParseOptions{
		DataRoot:      dataRoot,
		FilePath:      filePath,
		PreBufferSec:  cfg.Render.PreBufferSec,
		PostBufferSec: cfg.Render.PostBufferSec,
		Force:         force,
	})
	if err != nil {
		return err
	}

	outObj := parseCLISummary{
		DemoID:              sum.Demo.ID,
		MapName:             sum.Demo.MapName,
		TeamAName:           sum.Demo.TeamAName,
		TeamBName:           sum.Demo.TeamBName,
		TeamAScore:          sum.Demo.TeamAScore,
		TeamBScore:          sum.Demo.TeamBScore,
		TickRate:            sum.Demo.TickRate,
		HalftimeRound:       sum.Demo.HalftimeRound,
		RoundCount:          sum.RoundCount,
		PlayerCount:         sum.PlayerCount,
		EventCount:          sum.EventCount,
		HighlightCount:      sum.HighlightCount,
		InputDataCapability: sum.InputDataCapability,
		ReplayCachePath:     sum.ReplayCachePath,
		ReusedCache:         sum.ReusedCache,
		ElapsedMs:           time.Since(start).Milliseconds(),
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(outObj)
}

func runInspect(ctx context.Context, dataRoot string, demoID int64, rplPath string, roundNum uint32, out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")

	if rplPath != "" {
		hdr, entries, err := parser.ReadReplayIndex(rplPath)
		if err != nil {
			return err
		}
		if roundNum > 0 {
			_, rd, err := parser.ReadReplayRound(rplPath, roundNum)
			if err != nil {
				return err
			}
			return enc.Encode(map[string]any{
				"input_capability": parser.InputCapString(hdr.InputCap),
				"tick_rate":        hdr.TickRate,
				"round_number":     rd.RoundNumber,
				"start_tick":       rd.StartTick,
				"end_tick":         rd.EndTick,
				"frame_count":      len(rd.Frames),
				"grenade_count":    len(rd.Grenades),
				"smoke_count":      len(rd.Smokes),
				"inferno_count":    len(rd.Infernos),
				"input_edge_count": len(rd.InputEdges),
			})
		}
		return enc.Encode(map[string]any{
			"magic":            string(hdr.Magic[:]),
			"version":          hdr.Version,
			"input_capability": parser.InputCapString(hdr.InputCap),
			"tick_rate":        hdr.TickRate,
			"round_count":      hdr.RoundCount,
			"rounds":           entries,
		})
	}

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	demo, err := db.Demos().GetByID(ctx, demoID)
	if err != nil {
		return err
	}
	players, err := db.Analysis().ListMatchPlayers(ctx, demoID)
	if err != nil {
		return err
	}
	rounds, err := db.Analysis().ListMatchRounds(ctx, demoID)
	if err != nil {
		return err
	}
	clips, err := db.Analysis().ListHighlightClips(ctx, demoID)
	if err != nil {
		return err
	}
	return enc.Encode(map[string]any{
		"demo":       demo,
		"players":    players,
		"rounds":     rounds,
		"highlights": clips,
	})
}

type doctorSummary struct {
	Version       string             `json:"version"`
	DataRoot      string             `json:"data_root"`
	SchemaVersion int                `json:"schema_version"`
	Toolchain     config.ProbeReport `json:"toolchain"`
	IPCSelfCheck  string             `json:"ipc_self_check"`
	Platform      platformReport     `json:"platform"`
}

func runDoctor(ctx context.Context, dataRoot string, out io.Writer) error {
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths, DataRoot: dataRoot})

	db, err := storage.OpenDefault(dataRoot)
	if err != nil {
		return fmt.Errorf("storage open: %w", err)
	}
	defer db.Close()

	schemaVer, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("schema version: %w", err)
	}

	if err := runIPCSelfCheck(ctx, dataRoot); err != nil {
		return fmt.Errorf("ipc self-check: %w", err)
	}

	summary := doctorSummary{
		Version:       version.Summary("strikeye-cli"),
		DataRoot:      filepath.Clean(dataRoot),
		SchemaVersion: schemaVer,
		Toolchain:     probe,
		IPCSelfCheck:  "ok",
		Platform:      describePlatform(),
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
}

func runIPCSelfCheck(ctx context.Context, dataRoot string) error {
	lock := ipc.NewLockInDir(filepath.Join(dataRoot, "cache"), "doctor-selfcheck-lock")
	acquired, err := lock.TryAcquire()
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("expected doctor lock to be acquired")
	}
	defer func() {
		_ = lock.Release()
	}()

	sockAddr := filepath.Join(dataRoot, "doctor-ipc.sock")
	if os.PathSeparator == '\\' {
		sockAddr = fmt.Sprintf(`\\.\pipe\strikeye-doctor-%d`, time.Now().UnixNano())
	}
	ln, err := ipc.Listen(sockAddr)
	if err != nil {
		return fmt.Errorf("listen ipc: %w", err)
	}
	defer func() {
		_ = ln.Close()
		if os.PathSeparator != '\\' {
			_ = os.Remove(sockAddr)
		}
	}()

	srv := ipc.NewServer()
	srv.Register("agent.ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]string{"status": "pong"}, nil
	})

	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_ = srv.Serve(srvCtx, ln)
	}()

	conn, err := ipc.Dial(ctx, sockAddr)
	if err != nil {
		return fmt.Errorf("dial ipc: %w", err)
	}
	cli := ipc.NewClient(conn)
	defer cli.Close()

	var pong map[string]string
	if err := cli.Call(ctx, "agent.ping", nil, &pong); err != nil {
		return fmt.Errorf("call agent.ping: %w", err)
	}
	if pong["status"] != "pong" {
		return fmt.Errorf("unexpected ping reply: %+v", pong)
	}
	return nil
}

func runRenderDryRun(ctx context.Context, dataRoot string, demoID int64, dryRun bool, out io.Writer) error {
	cfg, _, err := config.Load(dataRoot)
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	probe := config.ProbeToolchain(config.ProbeOptions{Configured: cfg.Paths, DataRoot: dataRoot})
	cs2Root := ""
	if probe.CS2.Path != "" {
		cs2Root = filepath.Dir(filepath.Dir(filepath.Dir(probe.CS2.Path)))
	}
	cfgFiles, _ := recorder.DiscoverConfigFiles(probe.Steam.Path, cs2Root)

	sampleCFG, err := recorder.BuildBootCFG(recorder.BootCFGOptions{
		SessionID:     "dryrun",
		TempRenderDir: filepath.Join(dataRoot, "cache", "render_tmp", "dryrun"),
		DemoPlayArg:   "_strikeye_playback_dryrun.dem",
		DemoEndTick:   10000,
		TickRate:      64,
		Segments:      []recorder.RenderSegment{{TaskID: 1, SegIndex: 0, StartTick: 500, EndTick: 900, TargetSpecSlot: 1}},
	})
	if err != nil {
		return err
	}

	var queued []storage.RenderTask
	if demoID > 0 {
		db, err := storage.OpenDefault(dataRoot)
		if err != nil {
			return err
		}
		defer db.Close()
		all, err := db.RenderTasks().ListByStatus(ctx, "queued")
		if err != nil {
			return err
		}
		for _, t := range all {
			if t.DemoID == demoID {
				queued = append(queued, t)
			}
		}
	}

	var hlaeArgs []string
	if probe.HLAE.Path != "" && probe.AfxHookSource2.Path != "" && probe.CS2.Path != "" {
		hlaeArgs, _ = recorder.BuildHLAEArgs(recorder.LaunchOptions{
			SessionID:         "dryrun",
			HLAEExe:           probe.HLAE.Path,
			AfxHookSource2DLL: probe.AfxHookSource2.Path,
			CS2Exe:            probe.CS2.Path,
			Width:             cfg.Render.Width,
			Height:            cfg.Render.Height,
		})
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"dry_run":              dryRun,
		"policy":               cfg.Render.Policy,
		"probe":                probe,
		"discovered_cfg_files": cfgFiles,
		"queued_for_demo":      len(queued),
		"hlae_args":            hlaeArgs,
		"sample_boot_cfg":      sampleCFG,
	})
}

// stringListFlag collects repeated string flag values.
type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }

func (s *stringListFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}
