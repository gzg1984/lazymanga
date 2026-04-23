package normalization

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazymanga/models"
	"lazymanga/normalization/rulebook"
)

func TestDirectoryTransformHelpersRenameDirectoryAndWriteSidecar(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		Pattern:        `^\[(?P<circle>[^\]]+)\]\s*(?P<title>.+?)(?:\s+\((?P<year>\d{4})\))?(?:\s+\[(?P<karita_id>\d+)\])?$`,
		RenameTemplate: `${title}`,
		MetadataFile:   ".karita.meta.json",
		Metadata: map[string]string{
			"circle":    `${circle}`,
			"year":      `${year}`,
			"karita_id": `${karita_id}`,
		},
	}

	root := t.TempDir()
	originalName := "[CircleA] Sample Title (2024) [123456]"
	originalRelPath := filepath.ToSlash(filepath.Join("incoming", originalName))
	originalAbsPath := filepath.Join(root, filepath.FromSlash(originalRelPath))
	if err := os.MkdirAll(originalAbsPath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	for _, name := range []string{"001.jpg", "002.png"} {
		if err := os.WriteFile(filepath.Join(originalAbsPath, name), []byte("img"), 0o644); err != nil {
			t.Fatalf("write image failed: %v", err)
		}
	}

	targetName, captures, matched, err := applyDirectoryTransform(transform, originalName, originalRelPath)
	if err != nil {
		t.Fatalf("applyDirectoryTransform failed: %v", err)
	}
	if !matched {
		t.Fatalf("expected transform to match the karita-style name")
	}
	if targetName != "Sample Title" {
		t.Fatalf("expected clean title, got %q", targetName)
	}

	_, targetAbs, err := buildTargetPath(root, "incoming", targetName)
	if err != nil {
		t.Fatalf("buildTargetPath failed: %v", err)
	}
	finalAbs, finalRelPath, moved, err := relocateDirectoryWithUniqueTarget(originalAbsPath, targetAbs, root)
	if err != nil {
		t.Fatalf("relocateDirectoryWithUniqueTarget failed: %v", err)
	}
	if !moved || finalRelPath != "incoming/Sample Title" {
		t.Fatalf("unexpected relocation result moved=%v finalRelPath=%q", moved, finalRelPath)
	}

	payload := buildDirectorySidecarPayload("karita-folder", originalName, filepath.Base(finalRelPath), finalRelPath, transform, captures)
	if err := writeDirectorySidecar(finalAbs, transform.MetadataFile, payload); err != nil {
		t.Fatalf("writeDirectorySidecar failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(finalAbs, ".karita.meta.json"))
	if err != nil {
		t.Fatalf("read sidecar failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal sidecar failed: %v", err)
	}
	if decoded["normalized_name"] != "Sample Title" {
		t.Fatalf("unexpected normalized_name payload: %#v", decoded)
	}
	metadata, _ := decoded["metadata"].(map[string]any)
	if metadata["circle"] != "CircleA" || metadata["year"] != "2024" || metadata["karita_id"] != "123456" {
		t.Fatalf("unexpected metadata payload: %#v", decoded)
	}
}

func TestRelocateDirectoryWithUniqueTargetRemovesEmptySourceParents(t *testing.T) {
	root := t.TempDir()
	sourceRelPath := filepath.ToSlash(filepath.Join("incoming", "raw", "Publisher", "[CircleA] Sample Title"))
	sourceAbsPath := filepath.Join(root, filepath.FromSlash(sourceRelPath))
	if err := os.MkdirAll(sourceAbsPath, 0o755); err != nil {
		t.Fatalf("mkdir source path failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceAbsPath, "001.jpg"), []byte("img"), 0o644); err != nil {
		t.Fatalf("write image failed: %v", err)
	}

	_, targetAbs, err := buildTargetPath(root, "", "Sample Title")
	if err != nil {
		t.Fatalf("buildTargetPath failed: %v", err)
	}

	finalAbs, finalRelPath, moved, err := relocateDirectoryWithUniqueTarget(sourceAbsPath, targetAbs, root)
	if err != nil {
		t.Fatalf("relocateDirectoryWithUniqueTarget failed: %v", err)
	}
	if !moved {
		t.Fatal("expected directory to be moved")
	}
	if finalRelPath != "Sample Title" {
		t.Fatalf("expected relocated path to be flattened, got %q", finalRelPath)
	}
	if _, err := os.Stat(finalAbs); err != nil {
		t.Fatalf("expected relocated directory to exist, stat failed: %v", err)
	}

	for _, emptyDir := range []string{
		filepath.Join(root, "incoming", "raw", "Publisher"),
		filepath.Join(root, "incoming", "raw"),
		filepath.Join(root, "incoming"),
	} {
		if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
			t.Fatalf("expected empty source parent to be removed: %q err=%v", emptyDir, err)
		}
	}
}

func TestDirectoryTransformKaritaFilenameRecognizerExtractsMetadataAndFlattensPath(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RecognizerName:     "karita-manga-filename",
		RecognizerVersion:  "v1",
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"scanlator_group": `${scanlator_group}`,
			"event_code":      `${event_code}`,
			"author_name":     `${author_name}`,
			"author_alias":    `${author_alias}`,
			"original_work":   `${original_work}`,
		},
	}

	originalName := "【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)"
	originalRelPath := "漫画/漫画zip/H/LittleStory/C71-C87/" + originalName + "/" + originalName

	targetName, captures, matched, err := applyDirectoryTransform(transform, originalName, originalRelPath)
	if err != nil {
		t.Fatalf("applyDirectoryTransform failed: %v", err)
	}
	if !matched {
		t.Fatal("expected karita filename recognizer to match the sample path")
	}
	if targetName != "J's 2" {
		t.Fatalf("expected clean title, got %q", targetName)
	}
	if captures["target_path"] != "J's 2" {
		t.Fatalf("expected flattened target_path, got %#v", captures)
	}
	if captures["scanlator_group"] != "CE家族社" || captures["event_code"] != "C86" {
		t.Fatalf("expected scanlator/event metadata, got %#v", captures)
	}
	if captures["author_name"] != "牛乳のみお" || captures["author_alias"] != "牛乳屋さん" {
		t.Fatalf("expected author metadata, got %#v", captures)
	}
	if captures["original_work"] != "女子小学生はじめました" {
		t.Fatalf("expected original work metadata, got %#v", captures)
	}

	payload := buildDirectorySidecarPayload("karita-folder", originalName, targetName, captures["target_path"], transform, captures)
	metadata, _ := payload["metadata"].(map[string]string)
	if metadata["scanlator_group"] != "CE家族社" || metadata["author_alias"] != "牛乳屋さん" {
		t.Fatalf("unexpected payload metadata: %#v", payload)
	}
	pathParts, _ := payload["path_parts"].([]string)
	if len(pathParts) < 6 {
		t.Fatalf("expected source path parts to be preserved, got %#v", payload)
	}
}

func TestDirectoryTransformKaritaFilenameRecognizerExtractsSeriesNameFromParentPath(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RecognizerName:     "karita-manga-filename",
		RecognizerVersion:  "v1",
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"title":           `${title}`,
			"series_name":     `${series_name}`,
			"scanlator_group": `${scanlator_group}`,
			"event_code":      `${event_code}`,
			"circle_name":     `${circle_name}`,
			"original_work":   `${original_work}`,
		},
	}

	originalName := "【CE家族社】きょうこの日々 5日目!"
	originalRelPath := "漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C82) [とんずら道中] きょうこの日々 1~5 (東方Project)/【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)/" + originalName

	targetName, captures, matched, err := applyDirectoryTransform(transform, originalName, originalRelPath)
	if err != nil {
		t.Fatalf("applyDirectoryTransform failed: %v", err)
	}
	if !matched {
		t.Fatal("expected karita filename recognizer to infer series metadata from parent path")
	}
	if targetName != "きょうこの日々 5日目!" {
		t.Fatalf("expected leaf title to stay as the single-book title, got %q", targetName)
	}
	if captures["series_name"] != "きょうこの日々" {
		t.Fatalf("expected series_name to come from parent path, got %#v", captures)
	}
	if captures["scanlator_group"] != "CE家族社" || captures["event_code"] != "C82" {
		t.Fatalf("expected scanlator/event metadata from parent path, got %#v", captures)
	}
	if captures["circle_name"] != "とんずら道中" || captures["original_work"] != "東方Project" {
		t.Fatalf("expected circle/original work metadata from parent path, got %#v", captures)
	}

	payload := buildDirectorySidecarPayload("karita-folder", originalName, targetName, captures["target_path"], transform, captures)
	metadata, _ := payload["metadata"].(map[string]string)
	if metadata["series_name"] != "きょうこの日々" || metadata["title"] != "きょうこの日々 5日目!" {
		t.Fatalf("expected payload metadata to include both title and series_name, got %#v", payload)
	}
}

func TestDirectoryTransformKaritaFilenameRecognizerExtractsStructuredLeafTitleFromPath(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RecognizerName:     "karita-manga-filename",
		RecognizerVersion:  "v1",
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"title":           `${title}`,
			"series_name":     `${series_name}`,
			"scanlator_group": `${scanlator_group}`,
			"event_code":      `${event_code}`,
			"circle_name":     `${circle_name}`,
			"original_work":   `${original_work}`,
		},
	}

	originalName := "【CE家族社】(紅楼夢8) [とんずら道中] きょうこの日々 2日目! (東方Project)"
	originalRelPath := "漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C82) [とんずら道中] きょうこの日々 1~5 (東方Project)/【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)/" + originalName

	targetName, captures, matched, err := applyDirectoryTransform(transform, originalName, originalRelPath)
	if err != nil {
		t.Fatalf("applyDirectoryTransform failed: %v", err)
	}
	if !matched {
		t.Fatal("expected karita filename recognizer to parse the structured leaf path")
	}
	if targetName != "きょうこの日々 2日目!" {
		t.Fatalf("expected structured leaf title to be cleaned, got %q", targetName)
	}
	if captures["series_name"] != "きょうこの日々" {
		t.Fatalf("expected series_name to stay inferred from the parent path, got %#v", captures)
	}
	if captures["scanlator_group"] != "CE家族社" || captures["event_code"] != "紅楼夢8" {
		t.Fatalf("expected structured leaf metadata to be extracted, got %#v", captures)
	}
	if captures["circle_name"] != "とんずら道中" || captures["original_work"] != "東方Project" {
		t.Fatalf("expected circle/original work metadata from the leaf path, got %#v", captures)
	}
}

func TestDirectoryTransformKaritaFilenameRecognizerExtractsStructuredCurrentNameWithoutAuthor(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RecognizerName:     "karita-manga-filename",
		RecognizerVersion:  "v1",
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"title":           `${title}`,
			"scanlator_group": `${scanlator_group}`,
			"event_code":      `${event_code}`,
			"circle_name":     `${circle_name}`,
			"original_work":   `${original_work}`,
		},
	}

	originalName := "【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)"

	targetName, captures, matched, err := applyDirectoryTransform(transform, originalName, originalName)
	if err != nil {
		t.Fatalf("applyDirectoryTransform failed: %v", err)
	}
	if !matched {
		t.Fatal("expected karita filename recognizer to parse a structured current name without explicit author")
	}
	if targetName != "きょうこの日々" {
		t.Fatalf("expected title to be cleaned from the structured current name, got %q", targetName)
	}
	if captures["scanlator_group"] != "CE家族社" || captures["event_code"] != "C82" {
		t.Fatalf("expected scanlator/event metadata to be extracted, got %#v", captures)
	}
	if captures["circle_name"] != "とんずら道中" || captures["original_work"] != "東方Project" {
		t.Fatalf("expected circle/original work metadata to be extracted, got %#v", captures)
	}
}

func TestRenderDirectoryTransformFromMetadataUsesEditedTitleAndPathTemplate(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `作品/${original_work}/${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"title":           `${title}`,
			"scanlator_group": `${scanlator_group}`,
			"author_name":     `${author_name}`,
			"original_work":   `${original_work}`,
		},
	}

	mergedMetadata := map[string]string{
		"title":           "J's 3",
		"scanlator_group": "CE家族社",
		"author_name":     "牛乳のみお",
		"original_work":   "女子小学生はじめました",
	}

	targetName, captures, err := renderDirectoryTransformFromMetadata(transform, "J's 2", "J's 2", mergedMetadata)
	if err != nil {
		t.Fatalf("renderDirectoryTransformFromMetadata failed: %v", err)
	}
	if targetName != "J's 3" {
		t.Fatalf("expected edited title to drive rename target, got %q", targetName)
	}
	if captures["target_path"] != "作品/女子小学生はじめました/J's 3" {
		t.Fatalf("expected target_path to be re-rendered from edited metadata, got %#v", captures)
	}

	payload := buildDirectorySidecarPayload("karita-folder", "J's 2", targetName, captures["target_path"], transform, captures)
	metadata, _ := payload["metadata"].(map[string]string)
	if metadata["title"] != "J's 3" || metadata["original_work"] != "女子小学生はじめました" {
		t.Fatalf("expected sidecar payload to reflect edited metadata, got %#v", payload)
	}
}

func TestApplyDirectoryTransformWithAnalysisPreservesExistingMetadataOnNormalizedPath(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RecognizerName:     "karita-manga-filename",
		RecognizerVersion:  "v1",
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"title":           `${title}`,
			"series_name":     `${series_name}`,
			"scanlator_group": `${scanlator_group}`,
			"original_work":   `${original_work}`,
			"source_path":     `${path}`,
			"original_name":   `${original_name}`,
		},
	}

	existing := map[string]string{
		"title":           "J's 2",
		"series_name":     "J's",
		"scanlator_group": "CE家族社",
		"original_work":   "女子小学生はじめました",
		"source_path":     "漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)",
		"original_name":   "【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)",
	}

	targetName, captures, matched, err := applyDirectoryTransformWithAnalysis(transform, "J's 2", "整理后/J's 2", nil, existing)
	if err != nil {
		t.Fatalf("applyDirectoryTransformWithAnalysis failed: %v", err)
	}
	if !matched {
		t.Fatal("expected existing metadata to allow the normalized path to be re-rendered without losing metadata")
	}
	if targetName != "J's 2" {
		t.Fatalf("expected normalized title to stay stable, got %q", targetName)
	}
	if captures["scanlator_group"] != "CE家族社" || captures["original_work"] != "女子小学生はじめました" {
		t.Fatalf("expected existing metadata to be preserved, got %#v", captures)
	}
	if captures["source_path"] != existing["source_path"] || captures["original_name"] != existing["original_name"] {
		t.Fatalf("expected original provenance to be preserved, got %#v", captures)
	}

	payload := buildDirectorySidecarPayload("karita-folder", "J's 2", targetName, captures["target_path"], transform, captures)
	metadata, _ := payload["metadata"].(map[string]string)
	if metadata["source_path"] != existing["source_path"] || metadata["original_name"] != existing["original_name"] {
		t.Fatalf("expected payload metadata to keep original source path info, got %#v", payload)
	}
}

func TestApplyDirectoryTransformWithAnalysisUsesTextAnalyzerFallbackForBrokenBrackets(t *testing.T) {
	transform := &rulebook.DirectoryTransformSpec{
		RecognizerName:     "karita-manga-filename",
		RecognizerVersion:  "v1",
		RenameTemplate:     `${title}`,
		TargetPathTemplate: `${title}`,
		MetadataFile:       ".karita.meta.json",
		Metadata: map[string]string{
			"title":           `${title}`,
			"scanlator_group": `${scanlator_group}`,
			"original_work":   `${original_work}`,
			"source_path":     `${path}`,
			"original_name":   `${original_name}`,
		},
	}

	samples := []models.RepoISO{
		{
			FileName:     "J's 2",
			Path:         "J's 2",
			IsDirectory:  true,
			MetadataJSON: `{"title":"J's 2","scanlator_group":"CE家族社","event_code":"C86","circle_name":"牛乳屋さん","author_alias":"牛乳屋さん","original_work":"女子小学生はじめました","source_path":"漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)","original_name":"【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)"}`,
		},
		{
			FileName:     "J's 3",
			Path:         "J's 3",
			IsDirectory:  true,
			MetadataJSON: `{"title":"J's 3","scanlator_group":"CE家族社","event_code":"C87","circle_name":"牛乳屋さん","author_alias":"牛乳屋さん","original_work":"女子小学生はじめました","source_path":"漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C87) [牛乳屋さん (牛乳のみお)] J's 3 (女子小学生はじめました)","original_name":"【CE家族社】(C87) [牛乳屋さん (牛乳のみお)] J's 3 (女子小学生はじめました)"}`,
		},
	}
	model := buildRepoPathAnalysisModelFromRows(42, samples, time.Now())
	relativePath := "漫画/待整理/【CE家族社】[(C86] [牛乳屋さん (牛乳のみお)] J's 4 (女子小学生はじめました)"
	currentName := filepath.Base(filepath.FromSlash(relativePath))

	targetName, captures, matched, err := applyDirectoryTransformWithAnalysis(transform, currentName, relativePath, model, nil)
	if err != nil {
		t.Fatalf("applyDirectoryTransformWithAnalysis failed: %v", err)
	}
	if !matched {
		t.Fatal("expected text analyzer fallback to match broken-bracket leaf")
	}
	if targetName != "J's 4" {
		t.Fatalf("expected text analyzer fallback to infer title J's 4, got %q", targetName)
	}
	if captures["scanlator_group"] != "CE家族社" {
		t.Fatalf("expected scanlator group from analyzer fallback, got %#v", captures)
	}
	if captures["original_work"] != "女子小学生はじめました" {
		t.Fatalf("expected original work from analyzer fallback, got %#v", captures)
	}
	if captures["source_path"] != relativePath {
		t.Fatalf("expected fallback to preserve source_path, got %#v", captures)
	}
	if captures["original_name"] != currentName {
		t.Fatalf("expected fallback to preserve original_name, got %#v", captures)
	}
}
