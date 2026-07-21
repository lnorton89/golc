// path.go extends repository path containment (CONTEXT D-05/D-09) to
// indexed path *values* declared inside configuration — cache directories,
// mapping files, generated-output roots — not just the concern *files*
// load.go already resolves. A canonical key's declared value may name a
// repository-relative path that must never escape the repository through
// a lexical ".." segment, an absolute/drive-qualified prefix, or a
// symlink/reparse point anywhere along its resolved chain, even when the
// path's deepest segment does not exist yet (cache directories are
// created lazily by bootstrap, so requiring the leaf to exist first would
// reject perfectly safe declarations).
//
// ValidateConcernPath reuses the exact lexical check load.go already
// applies to concern file paths (assertRelativeConcernPath), so a path
// value and a concern file path can never silently diverge on what counts
// as "repository-relative". ResolveContainedPath adds the final on-disk
// containment check, tolerant of a not-yet-created leaf.
package projectconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateConcernPath rejects absolute, drive-qualified, parent, and dot
// path segments before any filesystem access — the lexical half of path
// containment, reused verbatim from concern-file path validation so a
// declared path value is held to exactly the same rule.
func ValidateConcernPath(relative string) error {
	return assertRelativeConcernPath(relative)
}

// ResolveContainedPath resolves a repository-relative path value against
// root and enforces final on-disk containment: every already-existing
// ancestor on the resolved chain must stay inside the resolved repository
// root once symlinks/reparse points are followed. The leaf itself (and any
// number of trailing segments) may not exist yet; containment is checked
// against the deepest existing ancestor and reapplied to the full
// candidate path once that ancestor is resolved, so a symlinked ancestor
// cannot smuggle a not-yet-created leaf outside the repository either.
func ResolveContainedPath(root, relative string) (string, error) {
	if err := ValidateConcernPath(relative); err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("GOLC_CONFIG_ROOT_MISSING: %q: %v", root, err)
	}
	rootWithSeparator := resolvedRoot + string(os.PathSeparator)
	joined := filepath.Join(resolvedRoot, filepath.FromSlash(strings.ReplaceAll(relative, "\\", "/")))

	candidate := joined
	suffix := ""
	for {
		resolved, evalErr := filepath.EvalSymlinks(candidate)
		if evalErr == nil {
			finalPath := resolved
			if suffix != "" {
				finalPath = filepath.Join(resolved, suffix)
			}
			if resolved != resolvedRoot && !strings.HasPrefix(resolved, rootWithSeparator) {
				return "", fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: %q resolves outside the repository", relative)
			}
			if finalPath != resolvedRoot && !strings.HasPrefix(finalPath, rootWithSeparator) {
				return "", fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: %q resolves outside the repository", relative)
			}
			return finalPath, nil
		}
		if !os.IsNotExist(evalErr) {
			return "", fmt.Errorf("GOLC_CONFIG_PATH_ESCAPE: %q: %v", relative, evalErr)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			// No ancestor on the whole chain exists. The lexical check
			// above already rejected every ../ and absolute escape, so the
			// joined path is safe to accept relative to resolvedRoot.
			return joined, nil
		}
		if suffix == "" {
			suffix = filepath.Base(candidate)
		} else {
			suffix = filepath.Join(filepath.Base(candidate), suffix)
		}
		candidate = parent
	}
}
