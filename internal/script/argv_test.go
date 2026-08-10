// argv_test.go covers internal/script/argv.go: buildRouteArgs' top-level
// dispatch (routing to the correct per-route builder, and the
// GOLC_SCRIPT_ROUTE_UNSUPPORTED/GOLC_SCRIPT_PARAMS_INVALID error paths),
// formatFloat's exact numeric-formatting behavior, and a representative
// cross-section of the ~35 per-route builders: at least one with a
// repeated/multi-value flag (buildPoolUpdateArgs, buildSceneLayerSetArgs,
// buildPlaybackBPMTapArgs), one with an optional parameter sometimes
// omitted from argv entirely (buildPoolCreateArgs, buildChaseUpdateArgs,
// buildArtnetConfigureArgs), one with numeric formatting via formatFloat
// (buildBlendCreateArgs, buildChaseCreateArgs), and builders with distinct
// non-mechanical logic (buildOperatorSurfaceAssignArgs' selector
// precedence switch, buildFixtureImportArgs' mutually-exclusive OFL/
// OFLFile switch). It is an internal (white-box) test package so it can
// call the unexported builders and routeArgvBuilders table directly,
// matching capability_test.go/host_test.go's existing white-box
// convention in this package.
package script

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- formatFloat ---------------------------------------------------------

func TestFormatFloat(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"zero", 0, "0"},
		{"positive-integer", 4, "4"},
		{"negative-integer", -4, "-4"},
		{"simple-fraction", 0.5, "0.5"},
		{"negative-fraction", -2.5, "-2.5"},
		{"trailing-zeros-trimmed", 1.0, "1"},
		{"many-decimal-places", 1.25, "1.25"},
		{"small-fraction-no-scientific-notation", 0.001, "0.001"},
		{"large-integer", 120000, "120000"},
		{"repeating-decimal-shortest-roundtrip", 1.0 / 3.0, "0.3333333333333333"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, formatFloat(tc.in))
		})
	}
}

// --- buildRouteArgs dispatch ----------------------------------------------

// TestBuildRouteArgs_Dispatch proves buildRouteArgs routes a representative
// sample of routes (covering every routeArgvBuilders category: shared
// shapes, pool, show, scene/blend/preset/chase/programmer, playback,
// operatorsurface, artnet, fixture, api-key) to the correct builder and
// produces that builder's real argv.
func TestBuildRouteArgs_Dispatch(t *testing.T) {
	const showPath = "/shows/current.golc"

	cases := []struct {
		name  string
		route string
		raw   string
		want  []string
	}{
		{
			name:  "shared-shape-show-only",
			route: "programmer inspect",
			raw:   `{}`,
			want:  []string{"--show", showPath},
		},
		{
			name:  "shared-shape-name-show",
			route: "deployment create",
			raw:   `{"name":"stage-left"}`,
			want:  []string{"stage-left", "--show", showPath},
		},
		{
			name:  "shared-shape-rename",
			route: "theme rename",
			raw:   `{"name":"warm","newName":"warm-v2"}`,
			want:  []string{"warm", "warm-v2", "--show", showPath},
		},
		{
			name:  "pool-create",
			route: "pool create",
			raw:   `{"name":"movers"}`,
			want:  []string{"movers", "--show", showPath},
		},
		{
			name:  "scene-create",
			route: "scene create",
			raw:   `{"name":"intro","bars":8}`,
			want:  []string{"intro", "--bars", "8", "--show", showPath},
		},
		{
			name:  "playback-bpm-set",
			route: "playback bpm set",
			raw:   `{"bpm":128}`,
			want:  []string{"128", "--show", showPath},
		},
		{
			name:  "operatorsurface-list-no-params",
			route: "operatorsurface list",
			raw:   `{}`,
			want:  []string{"--show", showPath},
		},
		{
			name:  "artnet-no-show-flag",
			route: "artnet interface list",
			raw:   `{}`,
			want:  []string{"--json"},
		},
		{
			name:  "fixture-no-show-flag",
			route: "fixture validate",
			raw:   `{"file":"fixtures/par.yaml"}`,
			want:  []string{"fixtures/par.yaml"},
		},
		{
			name:  "api-key-list",
			route: "api-key list",
			raw:   `{}`,
			want:  []string{"--show", showPath, "--json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildRouteArgs(tc.route, showPath, json.RawMessage(tc.raw))
			require.NoError(t, err, "buildRouteArgs(%q)", tc.route)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestBuildRouteArgs_UnsupportedRoute proves an unrecognized route fails
// loudly with GOLC_SCRIPT_ROUTE_UNSUPPORTED rather than silently producing
// an empty argv.
func TestBuildRouteArgs_UnsupportedRoute(t *testing.T) {
	got, err := buildRouteArgs("not a real route", "/shows/current.golc", json.RawMessage(`{}`))
	require.Nil(t, got)
	require.ErrorContains(t, err, "GOLC_SCRIPT_ROUTE_UNSUPPORTED")
	require.ErrorContains(t, err, `"not a real route"`)
}

// TestBuildRouteArgs_ParamsInvalid proves malformed Params JSON on an
// otherwise-recognized route surfaces decodeParams' GOLC_SCRIPT_PARAMS_INVALID
// error rather than a bare/unlabeled JSON error, and that buildRouteArgs
// returns a nil argv in that case.
func TestBuildRouteArgs_ParamsInvalid(t *testing.T) {
	got, err := buildRouteArgs("pool create", "/shows/current.golc", json.RawMessage(`{"name": not-json}`))
	require.Nil(t, got)
	require.ErrorContains(t, err, "GOLC_SCRIPT_PARAMS_INVALID")
}

// TestBuildRouteArgs_EmptyParamsDecodesToZeroValue proves decodeParams'
// "len(raw) == 0" branch is reachable through buildRouteArgs: an empty
// Params payload decodes to the zero value rather than erroring.
func TestBuildRouteArgs_EmptyParamsDecodesToZeroValue(t *testing.T) {
	got, err := buildRouteArgs("programmer inspect", "/shows/current.golc", json.RawMessage(``))
	require.NoError(t, err)
	require.Equal(t, []string{"--show", "/shows/current.golc"}, got)
}

// --- pool: optional parameter sometimes omitted ---------------------------

func TestBuildPoolCreateArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	t.Run("without-requires", func(t *testing.T) {
		got, err := buildPoolCreateArgs(showPath, json.RawMessage(`{"name":"movers"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"movers", "--show", showPath}, got)
	})

	t.Run("with-requires-joined-and-comma-separated", func(t *testing.T) {
		got, err := buildPoolCreateArgs(showPath, json.RawMessage(`{"name":"movers","requires":["pan","tilt"]}`))
		require.NoError(t, err)
		require.Equal(t, []string{"movers", "--requires", "pan,tilt", "--show", showPath}, got)
	})
}

// --- pool update: repeated flags plus conditional --json/--out -----------

func TestBuildPoolUpdateArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	t.Run("repeated-add-and-remove-flags-in-order", func(t *testing.T) {
		got, err := buildPoolUpdateArgs(showPath, json.RawMessage(`{"pool":"movers","add":["a","b"],"remove":["c"]}`))
		require.NoError(t, err)
		require.Equal(t, []string{
			"movers",
			"--add", "a",
			"--add", "b",
			"--remove", "c",
			"--json",
			"--show", showPath,
		}, got)
	})

	t.Run("json-always-added-when-out-omitted", func(t *testing.T) {
		got, err := buildPoolUpdateArgs(showPath, json.RawMessage(`{"pool":"movers"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"movers", "--json", "--show", showPath}, got)
	})

	t.Run("json-suppressed-when-out-set", func(t *testing.T) {
		got, err := buildPoolUpdateArgs(showPath, json.RawMessage(`{"pool":"movers","out":"plan.json"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"movers", "--out", "plan.json", "--show", showPath}, got)
	})

	t.Run("propagate-flag-included-when-set", func(t *testing.T) {
		got, err := buildPoolUpdateArgs(showPath, json.RawMessage(`{"pool":"movers","propagate":"preview"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"movers", "--propagate", "preview", "--json", "--show", showPath}, got)
	})
}

// --- scene layer set: multiple repeated flags + optional Ref/Disable -----

func TestBuildSceneLayerSetArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	t.Run("all-repeated-flags-and-ref-and-disable", func(t *testing.T) {
		raw := `{
			"scene": "intro",
			"kind": "base_look",
			"ref": "look-1",
			"instances": ["i1", "i2"],
			"pools": ["p1"],
			"groups": ["g1"],
			"fixtureRefs": ["f1", "f2"],
			"disable": true
		}`
		got, err := buildSceneLayerSetArgs(showPath, json.RawMessage(raw))
		require.NoError(t, err)
		require.Equal(t, []string{
			"intro", "--kind", "base_look",
			"--ref", "look-1",
			"--instance", "i1",
			"--instance", "i2",
			"--pool", "p1",
			"--group", "g1",
			"--fixture", "f1",
			"--fixture", "f2",
			"--disable",
			"--show", showPath,
		}, got)
	})

	t.Run("optional-fields-omitted-when-unset", func(t *testing.T) {
		got, err := buildSceneLayerSetArgs(showPath, json.RawMessage(`{"scene":"intro","kind":"chase"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"intro", "--kind", "chase", "--show", showPath}, got)
	})
}

// --- playback bpm tap: repeated --at flag ---------------------------------

func TestBuildPlaybackBPMTapArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	got, err := buildPlaybackBPMTapArgs(showPath, json.RawMessage(`{"at":["2024-01-01T00:00:00Z","2024-01-01T00:00:01Z"]}`))
	require.NoError(t, err)
	require.Equal(t, []string{
		"--at", "2024-01-01T00:00:00Z",
		"--at", "2024-01-01T00:00:01Z",
		"--show", showPath,
	}, got)
}

// --- blend/chase create: numeric formatting via formatFloat --------------

func TestBuildBlendCreateArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	t.Run("integer-duration-and-no-curve", func(t *testing.T) {
		got, err := buildBlendCreateArgs(showPath, json.RawMessage(`{"name":"fade","durationBars":4}`))
		require.NoError(t, err)
		require.Equal(t, []string{"fade", "--duration-bars", "4", "--show", showPath}, got)
	})

	t.Run("fractional-duration-and-curve", func(t *testing.T) {
		got, err := buildBlendCreateArgs(showPath, json.RawMessage(`{"name":"fade","durationBars":1.5,"curve":"ease_in"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"fade", "--duration-bars", "1.5", "--curve", "ease_in", "--show", showPath}, got)
	})
}

func TestBuildChaseCreateArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	got, err := buildChaseCreateArgs(showPath, json.RawMessage(`{"name":"sweep","unit":"beat","stepDuration":0.25}`))
	require.NoError(t, err)
	require.Equal(t, []string{"sweep", "--unit", "beat", "--step-duration", "0.25", "--show", showPath}, got)
}

// --- chase update: every field optional, including a real explicit-zero
// StepDuration (a *float64, unlike this file's other optional numeric
// fields, so JSON null/absent is distinguishable from an explicit 0) -----

func TestBuildChaseUpdateArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	t.Run("no-optional-fields-set", func(t *testing.T) {
		got, err := buildChaseUpdateArgs(showPath, json.RawMessage(`{"name":"sweep"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"sweep", "--show", showPath}, got)
	})

	t.Run("all-optional-fields-set", func(t *testing.T) {
		got, err := buildChaseUpdateArgs(showPath, json.RawMessage(`{"name":"sweep","newName":"sweep2","unit":"bar","stepDuration":2}`))
		require.NoError(t, err)
		require.Equal(t, []string{"sweep", "--name", "sweep2", "--unit", "bar", "--step-duration", "2", "--show", showPath}, got)
	})

	t.Run("explicit-zero-step-duration-is-distinguishable-from-omitted", func(t *testing.T) {
		// StepDuration is *float64 specifically so this case is
		// representable: a script explicitly requesting stepDuration: 0
		// now produces "--step-duration 0", not a silently-dropped flag.
		got, err := buildChaseUpdateArgs(showPath, json.RawMessage(`{"name":"sweep","stepDuration":0}`))
		require.NoError(t, err)
		require.Equal(t, []string{"sweep", "--step-duration", "0", "--show", showPath}, got)
	})

	t.Run("omitted-step-duration-still-produces-no-flag", func(t *testing.T) {
		got, err := buildChaseUpdateArgs(showPath, json.RawMessage(`{"name":"sweep","stepDuration":null}`))
		require.NoError(t, err)
		require.Equal(t, []string{"sweep", "--show", showPath}, got)
	})
}

// --- chase reorder: []int -> comma-joined string ---------------------------

func TestBuildChaseReorderArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	got, err := buildChaseReorderArgs(showPath, json.RawMessage(`{"name":"sweep","order":[2,0,1]}`))
	require.NoError(t, err)
	require.Equal(t, []string{"sweep", "--order", "2,0,1", "--show", showPath}, got)
}

// --- operatorsurface assign: fixed-precedence selector switch -------------

// TestBuildOperatorSurfaceAssignArgs_SelectorPrecedence proves the
// documented "first non-empty field in this fixed precedence order wins"
// contract: Scene > Layer > Master > Safety.
func TestBuildOperatorSurfaceAssignArgs_SelectorPrecedence(t *testing.T) {
	const showPath = "/shows/current.golc"

	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "scene-only",
			raw:  `{"surface":"deck-1","scene":"intro"}`,
			want: []string{"--surface", "deck-1", "--scene", "intro", "--show", showPath},
		},
		{
			name: "layer-only",
			raw:  `{"surface":"deck-1","layer":"intro:base_look"}`,
			want: []string{"--surface", "deck-1", "--layer", "intro:base_look", "--show", showPath},
		},
		{
			name: "master-only",
			raw:  `{"surface":"deck-1","master":"grand"}`,
			want: []string{"--surface", "deck-1", "--master", "grand", "--show", showPath},
		},
		{
			name: "safety-only",
			raw:  `{"surface":"deck-1","safety":"blackout"}`,
			want: []string{"--surface", "deck-1", "--safety", "blackout", "--show", showPath},
		},
		{
			name: "scene-wins-over-layer-master-safety-when-all-set",
			raw:  `{"surface":"deck-1","scene":"intro","layer":"intro:base_look","master":"grand","safety":"blackout"}`,
			want: []string{"--surface", "deck-1", "--scene", "intro", "--show", showPath},
		},
		{
			name: "layer-wins-over-master-safety-when-scene-unset",
			raw:  `{"surface":"deck-1","layer":"intro:base_look","master":"grand","safety":"blackout"}`,
			want: []string{"--surface", "deck-1", "--layer", "intro:base_look", "--show", showPath},
		},
		{
			name: "no-selector-set",
			raw:  `{"surface":"deck-1"}`,
			want: []string{"--surface", "deck-1", "--show", showPath},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildOperatorSurfaceAssignArgs(showPath, json.RawMessage(tc.raw))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// --- artnet configure: optional Port==0 omission + always-present bool ---

func TestBuildArtnetConfigureArgs(t *testing.T) {
	t.Run("port-omitted-when-zero", func(t *testing.T) {
		got, err := buildArtnetConfigureArgs("", json.RawMessage(`{"universe":1,"ip":"10.0.0.5","enabled":true}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--universe", "1", "--ip", "10.0.0.5", "--enabled", "true"}, got)
	})

	t.Run("port-included-when-set-and-pipe-appended", func(t *testing.T) {
		got, err := buildArtnetConfigureArgs("", json.RawMessage(`{"universe":1,"ip":"10.0.0.5","port":6454,"enabled":false,"pipe":"desk-1"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--universe", "1", "--ip", "10.0.0.5", "--port", "6454", "--enabled", "false", "--pipe", "desk-1"}, got)
	})
}

// --- artnet master set: --grand vs --group/--level branch, plus formatFloat

func TestBuildArtnetMasterSetArgs(t *testing.T) {
	t.Run("grand-branch-when-group-empty", func(t *testing.T) {
		got, err := buildArtnetMasterSetArgs("", json.RawMessage(`{"grand":0.75}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--grand", "0.75"}, got)
	})

	t.Run("group-and-level-branch-when-group-set", func(t *testing.T) {
		got, err := buildArtnetMasterSetArgs("", json.RawMessage(`{"group":"movers","level":0.5}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--group", "movers", "--level", "0.5"}, got)
	})
}

// --- fixture import: mutually-exclusive OFL/OFLFile switch ---------------

func TestBuildFixtureImportArgs(t *testing.T) {
	t.Run("ofl-identifier-branch", func(t *testing.T) {
		got, err := buildFixtureImportArgs("", json.RawMessage(`{"ofl":"chauvet/par","out":"fixtures/par.yaml"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--ofl", "chauvet/par", "--out", "fixtures/par.yaml"}, got)
	})

	t.Run("ofl-file-branch-with-mirror-and-allow-mirror", func(t *testing.T) {
		got, err := buildFixtureImportArgs("", json.RawMessage(`{"oflFile":"local.json","mirror":"https://mirror.example","allowMirror":true,"out":"fixtures/par.yaml"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--ofl-file", "local.json", "--mirror", "https://mirror.example", "--allow-mirror", "--out", "fixtures/par.yaml"}, got)
	})

	t.Run("neither-ofl-nor-oflfile-set-still-appends-out", func(t *testing.T) {
		got, err := buildFixtureImportArgs("", json.RawMessage(`{"out":"fixtures/par.yaml"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--out", "fixtures/par.yaml"}, got)
	})
}

// --- api-key revoke: optional trailing --json flag -------------------------

func TestBuildAPIKeyRevokeArgs(t *testing.T) {
	const showPath = "/shows/current.golc"

	t.Run("json-omitted-by-default", func(t *testing.T) {
		got, err := buildAPIKeyRevokeArgs(showPath, json.RawMessage(`{"id":"key-1"}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--id", "key-1", "--show", showPath}, got)
	})

	t.Run("json-included-when-requested", func(t *testing.T) {
		got, err := buildAPIKeyRevokeArgs(showPath, json.RawMessage(`{"id":"key-1","json":true}`))
		require.NoError(t, err)
		require.Equal(t, []string{"--id", "key-1", "--show", showPath, "--json"}, got)
	})
}

// --- decode error path, exercised directly against a per-route builder ---

// TestPerRouteBuilder_PropagatesDecodeError proves a per-route builder
// (not just buildRouteArgs' own dispatch) surfaces decodeParams' malformed-
// JSON error and returns a nil argv, using buildSceneCreateArgs as a
// representative example since it decodes into a struct with both a string
// and an int field.
func TestPerRouteBuilder_PropagatesDecodeError(t *testing.T) {
	got, err := buildSceneCreateArgs("/shows/current.golc", json.RawMessage(`{"bars": "not-a-number"}`))
	require.Nil(t, got)
	require.ErrorContains(t, err, "GOLC_SCRIPT_PARAMS_INVALID")
}

// --- routeArgvBuilders table completeness --------------------------------

// TestRouteArgvBuilders_AllRegisteredBuildersAreCallable proves every entry
// in routeArgvBuilders is a non-nil function, so buildRouteArgs can never
// panic on a nil-func lookup hit for a route the table claims to support.
func TestRouteArgvBuilders_AllRegisteredBuildersAreCallable(t *testing.T) {
	require.NotEmpty(t, routeArgvBuilders)
	for route, builder := range routeArgvBuilders {
		require.NotNil(t, builder, "route %q has a nil builder", route)
	}
}
