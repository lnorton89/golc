// package.go is the package command file: it owns the "package" scope and
// self-registers the exact route "package --foundation" (CONTEXT D-03,
// 01-RESEARCH.md's resolved Open Question 3): a deterministic Windows
// AMD64 developer-tool ZIP, canonical manifest, and SHA-256 checksum,
// built by internal/delivery.BuildFoundationBundle from the one
// authoritative graph inventory. This route makes no Wails/NSIS product
// packaging claim — that belongs to a later phase (01-CONTEXT.md Phase
// Boundary). Following router.go's dash-prefixed-word precedent
// (check.go's "--concern"/"--offline", test.go's "--quick"/"--scope"),
// the registered Route is the single word "package"; "--foundation" is
// strictly parsed inside the handler rather than being part of the route
// key itself, since a route word may never begin with a dash.
package command

import (
	"fmt"

	"github.com/lnorton89/golc/internal/delivery"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "package",
	Summary: "Deterministic developer-tool packaging (foundation ZIP only; no Wails/NSIS product build).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "package",
	Summary: "Build the deterministic Windows AMD64 foundation ZIP, canonical manifest, and SHA-256 checksum: " +
		"package --foundation.",
	Handler: runPackage,
})

// runPackage serves the self-registered "package" route. It accepts
// exactly one argument, "--foundation"; any other invocation is a usage
// error rather than a silent no-op product-packaging path.
func runPackage(request Request) Result {
	if len(request.Args) != 1 || request.Args[0] != "--foundation" {
		return Result{ExitCode: 2, Stderr: []byte(
			"GOLC_PACKAGE_USAGE: usage: package --foundation\n")}
	}
	return runPackageFoundation(request.Root)
}

// runPackageFoundation builds the deterministic foundation bundle and
// writes it to the fixed output location under dist/foundation, replacing
// any prior output there so repeated invocations are directly comparable
// byte-for-byte (tests/acceptance/offline.ps1 -Mode package).
func runPackageFoundation(root string) Result {
	bundle, err := delivery.BuildFoundationBundle(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte("GOLC_PACKAGE_FOUNDATION: " + err.Error() + "\n")}
	}

	paths := delivery.DefaultFoundationOutputPaths(root)
	if err := delivery.WriteFoundationBundle(bundle, paths); err != nil {
		return Result{ExitCode: 1, Stderr: []byte("GOLC_PACKAGE_FOUNDATION: " + err.Error() + "\n")}
	}

	output := fmt.Sprintf(
		"GOLC package --foundation: wrote %d files (developer tooling only; no Wails/NSIS product claim). "+
			"zip sha256 %s, manifest sha256 %s.\n",
		len(bundle.Manifest.Files), bundle.ZIPChecksum, bundle.ManifestChecksum)
	return Result{Stdout: []byte(output)}
}
