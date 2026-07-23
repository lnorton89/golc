// show.go registers the explicit "show open"/"show save"/"show save-as"
// routes on the "show" scope deployment.go already declares (SHOW-02) --
// this file must never call MustDeclareScope("show") again, or the
// registry panics with GOLC_SCOPE_DUPLICATE at init. "show open" is where
// SHOW-04's interrupted-session recovery is surfaced: after Load, it
// queries internal/show.DetectRecoveryPoints and, when a newer recovery
// point exists, offers (never auto-applies) inspect/accept/discard next
// steps -- mirroring the preview-then-confirm shape "pool update"/"pool
// apply" already establish (CONTEXT D-07; the recovery prohibition: MUST
// NOT auto-apply, silently overwrite, or discard the user's
// explicitly-saved .golc contents).
package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lnorton89/golc/internal/show"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route: "show open",
	Summary: "Open a ShowState document for edit, offering (never auto-applying) any interrupted-session " +
		"recovery point found: show open --show <path> [--accept-recovery <id>] [--discard-recovery].",
	Handler: runShowOpen,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "show save",
	Summary: "Load and re-save a ShowState document, writing a fresh recovery point: show save --show <path>.",
	Handler: runShowSave,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "show save-as",
	Summary: "Load a ShowState document read-only and save it to a new path without mutating the source: show save-as --show <src> --to <dest>.",
	Handler: runShowSaveAs,
})

// parseShowSaveArgs accepts exactly a required "--show <path>" (both
// --flag value and --flag=value forms), rejecting anything else.
func parseShowSaveArgs(usage string, args []string) (showPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_SHOW_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", fmt.Errorf("GOLC_SHOW_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", fmt.Errorf("GOLC_SHOW_USAGE: --show is required; usage: %s", usage)
	}
	return showPath, nil
}

// runShowSave serves the self-registered "show save" route: load the
// ShowState at --show and save it right back to the same path, which
// (via show.Save) both bumps Revision and writes a fresh recovery point in
// the same transaction (CONTEXT D-04).
func runShowSave(request Request) Result {
	showPath, err := parseShowSaveArgs("show save --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	// show.Save never mutates its State argument in place -- callers
	// observe the bumped Revision by loading again (store.go's own Save
	// doc comment).
	saved, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SHOW_SAVED: %s (revision %d)\n", showPath, saved.Revision))}
}

// showSaveAsArgs is the parsed shape of one "show save-as" invocation.
type showSaveAsArgs struct {
	showPath string
	toPath   string
}

// parseShowSaveAsArgs accepts exactly the required "--show <src>" and
// "--to <dest>" pair (both --flag value and --flag=value forms), rejecting
// anything else.
func parseShowSaveAsArgs(usage string, args []string) (showSaveAsArgs, error) {
	parsed := showSaveAsArgs{}
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return showSaveAsArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		case argument == "--to":
			if i+1 >= len(args) {
				return showSaveAsArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --to requires a path; usage: %s", usage)
			}
			parsed.toPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--to="):
			parsed.toPath = strings.TrimPrefix(argument, "--to=")
			i++
		default:
			return showSaveAsArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.showPath == "" || parsed.toPath == "" {
		return showSaveAsArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --show and --to are required; usage: %s", usage)
	}
	return parsed, nil
}

// runShowSaveAs serves the self-registered "show save-as" route: load the
// ShowState at --show (read-only -- the source is never re-saved) and save
// it to --to, resolved through the same single resolveWritablePath
// root-relative-vs-absolute rule every other write destination in this
// package already uses (internal/show.Save resolves an already-absolute
// path unchanged, so pre-resolving here does not invent a second rule).
func runShowSaveAs(request Request) Result {
	usage := "show save-as --show <src> --to <dest>"
	parsed, err := parseShowSaveAsArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	destPath := resolveWritablePath(request.Root, parsed.toPath)
	if err := show.Save(request.Root, destPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SHOW_SAVED_AS: %s -> %s\n", parsed.showPath, destPath))}
}

// showOpenArgs is the parsed shape of one "show open" invocation.
// acceptRecovery/discardRecovery are mutually exclusive -- an invocation
// either accepts one offered recovery point, discards every offered
// recovery point, or does neither and only reports the offer.
// confirmMigration is independent of both: it only takes effect when
// show.Load reports ErrSchemaMigrationRequired (CONTEXT D-08).
type showOpenArgs struct {
	showPath         string
	acceptRecovery   bool
	acceptRecoveryID int
	discardRecovery  bool
	confirmMigration bool
}

// parseShowOpenArgs accepts a required "--show <path>", an optional
// "--accept-recovery <id>" (an integer recovery point id), an optional
// "--discard-recovery" flag, and an optional "--confirm-migration" flag
// (both --flag value and --flag=value forms for the id), rejecting
// anything else and rejecting --accept-recovery/--discard-recovery
// together.
func parseShowOpenArgs(usage string, args []string) (showOpenArgs, error) {
	parsed := showOpenArgs{}
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		case argument == "--accept-recovery":
			if i+1 >= len(args) {
				return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --accept-recovery requires an id; usage: %s", usage)
			}
			id, convErr := strconv.Atoi(args[i+1])
			if convErr != nil {
				return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --accept-recovery requires an integer id, got %q; usage: %s", args[i+1], usage)
			}
			parsed.acceptRecovery = true
			parsed.acceptRecoveryID = id
			i += 2
		case strings.HasPrefix(argument, "--accept-recovery="):
			raw := strings.TrimPrefix(argument, "--accept-recovery=")
			id, convErr := strconv.Atoi(raw)
			if convErr != nil {
				return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --accept-recovery requires an integer id, got %q; usage: %s", raw, usage)
			}
			parsed.acceptRecovery = true
			parsed.acceptRecoveryID = id
			i++
		case argument == "--discard-recovery":
			parsed.discardRecovery = true
			i++
		case argument == "--confirm-migration":
			parsed.confirmMigration = true
			i++
		default:
			return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.showPath == "" {
		return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --show is required; usage: %s", usage)
	}
	if parsed.acceptRecovery && parsed.discardRecovery {
		return showOpenArgs{}, fmt.Errorf("GOLC_SHOW_USAGE: --accept-recovery and --discard-recovery are mutually exclusive; usage: %s", usage)
	}
	return parsed, nil
}

// recoveryOfferMessage renders the GOLC_SHOW_RECOVERY_AVAILABLE offer:
// point count and revisions, plus the exact next-step invocations an
// operator can run -- inspect (this same "show open"), accept, or discard.
// It never applies anything itself.
func recoveryOfferMessage(showPath string, points []show.RecoveryPoint) string {
	revisions := make([]string, 0, len(points))
	for _, point := range points {
		revisions = append(revisions, strconv.Itoa(point.Revision))
	}
	return fmt.Sprintf(
		"GOLC_SHOW_RECOVERY_AVAILABLE: %d recovery point(s) found (revisions %s); "+
			"inspect again with \"show open --show %s\", accept one with \"show open --show %s --accept-recovery <id>\", "+
			"or discard every offered point with \"show open --show %s --discard-recovery\"\n",
		len(points), strings.Join(revisions, ", "), showPath, showPath, showPath)
}

// runShowOpen serves the self-registered "show open" route: load the
// ShowState at --show for edit (show.Load, which hard-refuses a
// newer-than-supported schema_version as GOLC_SHOW_SCHEMA_TOO_NEW per
// D-10 -- read-only inspect/export/diagnose for that case is a separate
// surface, internal/command/show_diagnose.go). An older-than-supported
// schema_version (show.ErrSchemaMigrationRequired) is detected and
// reported as GOLC_SHOW_MIGRATION_REQUIRED, touching nothing, unless the
// caller passes --confirm-migration -- only then does this handler call
// show.Migrate (verifiedBackup -> migrate-temp -> re-validate -> atomic
// replace, Plan 03) and re-Load the migrated result, reporting
// GOLC_SHOW_MIGRATED with the backup path (CONTEXT D-08; mirrors
// runPoolApply's detect-separately-from-mutate shape -- no code path here
// migrates without the explicit token). After Load (or a successful
// migration's re-Load), it queries show.DetectRecoveryPoints. A newer
// interrupted-session recovery point is only ever offered here; it is
// applied only via an explicit --accept-recovery <id> (which must name one
// of the currently offered points) or removed only via an explicit
// --discard-recovery -- open with neither flag never mutates anything
// beyond the read Load (and, when confirmed, the migration) already
// performs.
func runShowOpen(request Request) Result {
	usage := "show open --show <path> [--accept-recovery <id>] [--discard-recovery] [--confirm-migration]"
	parsed, err := parseShowOpenArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	var migrationNotice string
	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		var tooNew show.ErrSchemaTooNew
		if errors.As(err, &tooNew) {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		var migrationRequired show.ErrSchemaMigrationRequired
		if errors.As(err, &migrationRequired) {
			if !parsed.confirmMigration {
				return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
					"GOLC_SHOW_MIGRATION_REQUIRED: %s requires migration from schema_version %d to %d before it can be opened; "+
						"a verified backup will be made first -- re-run with \"show open --show %s --confirm-migration\" to proceed\n",
					parsed.showPath, migrationRequired.Found, migrationRequired.Supported, parsed.showPath))}
			}
			backupPath, migrateErr := show.Migrate(request.Root, parsed.showPath)
			if migrateErr != nil {
				return Result{ExitCode: 1, Stderr: []byte(migrateErr.Error() + "\n")}
			}
			migrationNotice = fmt.Sprintf("GOLC_SHOW_MIGRATED: %s migrated to schema_version %d (backup: %s)\n",
				parsed.showPath, migrationRequired.Supported, backupPath)
			state, err = show.Load(request.Root, parsed.showPath)
			if err != nil {
				return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
			}
		} else {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
	}

	points, err := show.DetectRecoveryPoints(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	var output strings.Builder
	if migrationNotice != "" {
		output.WriteString(migrationNotice)
	}
	if len(points) > 0 {
		output.WriteString(recoveryOfferMessage(parsed.showPath, points))
	}

	switch {
	case parsed.acceptRecovery:
		offered := false
		for _, point := range points {
			if point.ID == parsed.acceptRecoveryID {
				offered = true
				break
			}
		}
		if !offered {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
				"GOLC_SHOW_RECOVERY_NOT_FOUND: recovery point %d is not currently offered\n", parsed.acceptRecoveryID))}
		}
		if err := show.AcceptRecoveryPoint(request.Root, parsed.showPath, parsed.acceptRecoveryID); err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		fmt.Fprintf(&output, "GOLC_SHOW_RECOVERY_ACCEPTED: recovery point %d applied\n", parsed.acceptRecoveryID)
		state, err = show.Load(request.Root, parsed.showPath)
		if err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
	case parsed.discardRecovery:
		if err := show.DiscardRecoveryPoints(request.Root, parsed.showPath); err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		output.WriteString("GOLC_SHOW_RECOVERY_DISCARDED: offered recovery point(s) removed\n")
	}

	fmt.Fprintf(&output, "GOLC_SHOW_OPENED: %s (schema_version %d, revision %d)\n", parsed.showPath, state.SchemaVersion, state.Revision)
	return Result{Stdout: []byte(output.String())}
}
