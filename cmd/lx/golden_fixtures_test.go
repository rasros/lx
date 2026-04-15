package main

import (
	"testing"

	gf "github.com/rasros/lx/cmd/lx/goldenfixtures"
)

func setupWalkFixture(t *testing.T) string       { return gf.SetupWalkFixture(t) }
func setupFormattingFixture(t *testing.T) string { return gf.SetupFormattingFixture(t) }
func setupSectionsFixture(t *testing.T) string   { return gf.SetupSectionsFixture(t) }
func setupFilteringFixture(t *testing.T) string  { return gf.SetupFilteringFixture(t) }
func setupSlicingFixture(t *testing.T) string    { return gf.SetupSlicingFixture(t) }
func setupSymlinksFixture(t *testing.T) string   { return gf.SetupSymlinksFixture(t) }
func setupErrorsFixture(t *testing.T) string     { return gf.SetupErrorsFixture(t) }
func setupStatsFixture(t *testing.T) string      { return gf.SetupStatsFixture(t) }
func setupDetectionFixture(t *testing.T) string  { return gf.SetupDetectionFixture(t) }
func setupConfigFixture(t *testing.T) string     { return gf.SetupConfigFixture(t) }
func setupComplexFixture(t *testing.T) string    { return gf.SetupComplexFixture(t) }
func setupArchiveFixture(t *testing.T) string    { return gf.SetupArchiveFixture(t) }
func setupDocumentsFixture(t *testing.T) string  { return gf.SetupDocumentsFixture(t) }
func setupSkeletonFixture(t *testing.T) string   { return gf.SetupSkeletonFixture(t) }
func setupRelativePathsFixture(t *testing.T) (string, string) {
	return gf.SetupRelativePathsFixture(t)
}
