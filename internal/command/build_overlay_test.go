package command

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectBuildGoArgumentsDialogProofOverlay(t *testing.T) {
	root := t.TempDir()
	arguments := []string{"build", "-v", "./..."}

	t.Setenv(dialogProofGoOverlayEnvName, "")
	t.Setenv(dialogProofGoModfileEnvName, "")
	require.Equal(t, arguments, projectBuildGoArguments(arguments))

	overlay := filepath.Join(root, "path with spaces", "overlay.json")
	modfile := filepath.Join(root, "path with spaces", "proof.mod")
	t.Setenv(dialogProofGoOverlayEnvName, "  "+overlay+"  ")
	require.Equal(t, arguments, projectBuildGoArguments(arguments))
	t.Setenv(dialogProofGoModfileEnvName, "  "+modfile+"  ")
	require.Equal(t, []string{"build", "-modfile=" + modfile, "-overlay=" + overlay, "-tags=" + dialogProofGoOverlayBuildTag, "-v", "./..."}, projectBuildGoArguments(arguments))
	require.Equal(t,
		[]string{"build", "-modfile=" + modfile, "-overlay=" + overlay, "-tags", "desktop,production," + dialogProofGoOverlayBuildTag, "./cmd/golc-desktop"},
		projectBuildGoArguments([]string{"build", "-tags", "desktop,production", "./cmd/golc-desktop"}),
	)
	require.Equal(t, []string{"list", "./..."}, projectBuildGoArguments([]string{"list", "./..."}))
	require.Equal(t, []string{"build", "-v", "./..."}, arguments)
}
