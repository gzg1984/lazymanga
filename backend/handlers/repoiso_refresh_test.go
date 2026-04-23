package handlers

import (
	"strings"
	"testing"

	"lazymanga/models"
	"lazymanga/normalization"
)

func TestDetectDirectoryRefreshChangesReportsPathAndSizeUpdates(t *testing.T) {
	row := &models.RepoISO{
		FileName:    "J's 2",
		Path:        "J's 2",
		IsDirectory: true,
	}

	pathMoved, sizeUpdated := detectDirectoryRefreshChanges(
		"【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)",
		"【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)",
		models.UnknownRepoISOSizeBytes,
		row,
		12345,
	)
	if !pathMoved {
		t.Fatal("expected renamed directory path to be reported as moved")
	}
	if !sizeUpdated {
		t.Fatal("expected recursive directory size change to be reported")
	}
}

func TestCoveredScannedPathsForExistingRowIncludesOldAndNewPathsWithoutDuplicates(t *testing.T) {
	row := &models.RepoISO{Path: "normalized/J's 2", FileName: "J's 2", IsDirectory: true}
	coveredPaths := coveredScannedPathsForExistingRow(
		"incoming/raw/【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)",
		row,
		true,
	)
	if len(coveredPaths) != 2 {
		t.Fatalf("expected exactly old and new paths to be covered, got %#v", coveredPaths)
	}
	if coveredPaths[0] != "incoming/raw/【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)" {
		t.Fatalf("unexpected original covered path: %#v", coveredPaths)
	}
	if coveredPaths[1] != "normalized/J's 2" {
		t.Fatalf("unexpected refreshed covered path: %#v", coveredPaths)
	}

	unchanged := coveredScannedPathsForExistingRow("same/path", &models.RepoISO{Path: "same/path"}, true)
	if len(unchanged) != 1 || unchanged[0] != "same/path" {
		t.Fatalf("expected duplicate paths to collapse into one entry, got %#v", unchanged)
	}
}

func TestBuildRepoISORefreshAnalysisPathStripsFileExtension(t *testing.T) {
	analysisPath := buildRepoISORefreshAnalysisPath(models.RepoISO{
		Path:     "archives/えんこーせい! [中国翻訳].zip",
		FileName: "えんこーせい! [中国翻訳].zip",
	}, repoISOItemKindArchive, "archives")
	if analysisPath != "えんこーせい! [中国翻訳]" {
		t.Fatalf("expected analysis path without extension, got %q", analysisPath)
	}

	directoryPath := buildRepoISORefreshAnalysisPath(models.RepoISO{
		Path:        "incoming/【CE家族社】 J's 4",
		FileName:    "【CE家族社】 J's 4",
		IsDirectory: true,
	}, "", "")
	if directoryPath != "incoming/【CE家族社】 J's 4" {
		t.Fatalf("expected directory analysis path to remain unchanged, got %q", directoryPath)
	}
}

func TestBuildRepoISORefreshProposedMetadataAddsArchiveBackfillAndInferredTitle(t *testing.T) {
	row := models.RepoISO{
		Path:     "archives/えんこーせい! [中国翻訳].zip",
		FileName: "えんこーせい! [中国翻訳].zip",
	}
	guess := normalization.AnalyzePathMetadata(nil, buildRepoISORefreshAnalysisPath(row, repoISOItemKindArchive, "archives"))
	guess.Metadata["source_path"] = sanitizeStoredSourceRelativePath(row.Path)
	guess.Metadata["original_name"] = sanitizeStoredSourcePathSegment(row.FileName)

	metadata, changedFields, changes, err := buildRepoISORefreshProposedMetadata(row, repoISOItemKindArchive, guess.Metadata)
	if err != nil {
		t.Fatalf("buildRepoISORefreshProposedMetadata failed: %v", err)
	}
	if len(changedFields) == 0 {
		t.Fatal("expected archive refresh proposal changes")
	}
	if metadata["item_kind"] != repoISOItemKindArchive {
		t.Fatalf("expected archive item_kind, got %#v", metadata["item_kind"])
	}
	if metadata["archive_storage_path"] != row.Path {
		t.Fatalf("expected archive_storage_path to be backfilled, got %#v", metadata["archive_storage_path"])
	}
	if strings.TrimSpace(toTestString(metadata["title"])) == "" {
		t.Fatalf("expected inferred title in proposal, got %#v", metadata["title"])
	}
	if _, ok := changes["title"]; !ok {
		t.Fatalf("expected title change in proposal, got %#v", changes)
	}
	if got := toTestString(metadata["original_name"]); got != "えんこーせい! [中国翻訳].zip" {
		t.Fatalf("expected original_name with extension preserved, got %q", got)
	}
	if got := toTestString(changes["item_kind"].To); got != repoISOItemKindArchive {
		t.Fatalf("expected item_kind change to archive, got %q", got)
	}
}

func TestBuildRepoISORefreshProposedMetadataDoesNotOverwriteExistingMetadata(t *testing.T) {
	row := models.RepoISO{
		Path:         "archives/えんこーせい! [中国翻訳].zip",
		FileName:     "えんこーせい! [中国翻訳].zip",
		MetadataJSON: `{"item_kind":"archive","title":"Manual Title"}`,
	}
	guess := map[string]string{
		"title":         "Auto Title",
		"source_path":   row.Path,
		"original_name": row.FileName,
	}

	metadata, changedFields, _, err := buildRepoISORefreshProposedMetadata(row, repoISOItemKindArchive, guess)
	if err != nil {
		t.Fatalf("buildRepoISORefreshProposedMetadata failed: %v", err)
	}
	if got := toTestString(metadata["title"]); got != "Manual Title" {
		t.Fatalf("expected existing title to win, got %q", got)
	}
	for _, field := range changedFields {
		if field == "title" {
			t.Fatalf("did not expect title to be proposed for overwrite: %#v", changedFields)
		}
	}
}

func TestBuildRepoISORefreshMetadataAnalysisWithContextReportsNoProposal(t *testing.T) {
	row := models.RepoISO{
		Path:         "archives/えんこーせい! [中国翻訳].zip",
		FileName:     "えんこーせい! [中国翻訳].zip",
		MetadataJSON: `{"item_kind":"archive","title":"えんこーせい! [中国翻訳]","archive_storage_path":"archives/えんこーせい! [中国翻訳].zip","source_path":"archives/えんこーせい! [中国翻訳].zip","original_name":"えんこーせい! [中国翻訳].zip"}`,
	}
	ctx := &repoISORefreshProposalContext{
		EditorMode:    manualEditorModeMetadata,
		ItemKind:      repoISOItemKindArchive,
		ArchiveSubdir: "archives",
	}

	analysis, proposal, err := buildRepoISORefreshMetadataAnalysisWithContext(ctx, row, "2026-04-23T00:00:00Z")
	if err != nil {
		t.Fatalf("buildRepoISORefreshMetadataAnalysisWithContext failed: %v", err)
	}
	if proposal != nil {
		t.Fatalf("expected no proposal, got %#v", proposal)
	}
	if analysis == nil {
		t.Fatal("expected analysis result")
	}
	if !analysis.Attempted {
		t.Fatal("expected analysis attempt to be recorded")
	}
	if analysis.Status != "no-proposal" {
		t.Fatalf("expected no-proposal status, got %q", analysis.Status)
	}
	if analysis.Reason != "no-inferred-metadata" {
		t.Fatalf("unexpected analysis reason: %q", analysis.Reason)
	}
	if analysis.AnalyzedAt != "2026-04-23T00:00:00Z" {
		t.Fatalf("unexpected analyzed_at: %q", analysis.AnalyzedAt)
	}
	if analysis.ProposalAvailable {
		t.Fatal("expected proposal_available to be false")
	}
	if analysis.AnalysisPath != "えんこーせい! [中国翻訳]" {
		t.Fatalf("unexpected analysis path: %q", analysis.AnalysisPath)
	}
	if len(analysis.DetectedFields) != 0 {
		t.Fatalf("expected no semantic detected fields, got %#v", analysis.DetectedFields)
	}
	if len(analysis.BlockedFields) != 0 {
		t.Fatalf("expected no semantic blocked fields, got %#v", analysis.BlockedFields)
	}
}

func TestNonEmptyRepoISORefreshGuessFieldsIgnoresTechnicalFields(t *testing.T) {
	fields := nonEmptyRepoISORefreshGuessFields(map[string]string{
		"title":                "口袋妖怪",
		"source_path":          "archives/example.zip",
		"archive_storage_path": "archives/example.zip",
		"original_name":        "example.zip",
	})
	if len(fields) != 1 || fields[0] != "title" {
		t.Fatalf("expected only semantic title field, got %#v", fields)
	}
}

func TestBuildRepoISORefreshProposedMetadataHidesNonProposalVisibleChanges(t *testing.T) {
	row := models.RepoISO{}
	guess := map[string]string{
		"title":           "Sample Title",
		"normalized_name": "sample-title",
	}

	metadata, changedFields, changes, err := buildRepoISORefreshProposedMetadata(row, "", guess)
	if err != nil {
		t.Fatalf("buildRepoISORefreshProposedMetadata failed: %v", err)
	}
	if got := toTestString(metadata["normalized_name"]); got != "sample-title" {
		t.Fatalf("expected metadata to retain normalized_name payload, got %#v", metadata)
	}
	if len(changedFields) != 1 || changedFields[0] != "title" {
		t.Fatalf("expected only visible title change, got %#v", changedFields)
	}
	if _, ok := changes["normalized_name"]; ok {
		t.Fatalf("did not expect normalized_name in visible change set: %#v", changes)
	}
	if _, ok := changes["title"]; !ok {
		t.Fatalf("expected title in visible change set: %#v", changes)
	}
}

func toTestString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
