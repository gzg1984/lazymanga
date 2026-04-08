package normalization

import "testing"

func TestEvaluateDirectoryNameRecognizerWithExactUserPath(t *testing.T) {
	currentName := "【CE家族社】きょうこの日々 5日目!"
	relativePath := "漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C82) [とんずら道中] きょうこの日々 1~5 (東方Project)/【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)/【CE家族社】きょうこの日々 5日目! "

	captures, matched, err := evaluateDirectoryNameRecognizer("karita-manga-filename", "v1", currentName, relativePath)
	if err != nil {
		t.Fatalf("evaluateDirectoryNameRecognizer failed: %v", err)
	}
	if !matched {
		t.Fatal("expected exact user path to match karita-manga-filename@v1")
	}

	checks := map[string]string{
		"title":           "きょうこの日々 5日目!",
		"series_name":     "きょうこの日々",
		"scanlator_group": "CE家族社",
		"event_code":      "C82",
		"circle_name":     "とんずら道中",
		"original_work":   "東方Project",
	}
	for key, want := range checks {
		if got := captures[key]; got != want {
			t.Fatalf("expected %s=%q, got %q (captures=%#v)", key, want, got, captures)
		}
	}
}

func TestEvaluateDirectoryNameRecognizerWithStructuredLeafPath(t *testing.T) {
	currentName := "【CE家族社】(紅楼夢8) [とんずら道中] きょうこの日々 2日目! (東方Project)"
	relativePath := "漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C82) [とんずら道中] きょうこの日々 1~5 (東方Project)/【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)/" + currentName

	captures, matched, err := evaluateDirectoryNameRecognizer("karita-manga-filename", "v1", currentName, relativePath)
	if err != nil {
		t.Fatalf("evaluateDirectoryNameRecognizer failed: %v", err)
	}
	if !matched {
		t.Fatal("expected structured leaf path to match karita-manga-filename@v1")
	}

	checks := map[string]string{
		"title":           "きょうこの日々 2日目!",
		"series_name":     "きょうこの日々",
		"scanlator_group": "CE家族社",
		"event_code":      "紅楼夢8",
		"circle_name":     "とんずら道中",
		"original_work":   "東方Project",
	}
	for key, want := range checks {
		if got := captures[key]; got != want {
			t.Fatalf("expected %s=%q, got %q (captures=%#v)", key, want, got, captures)
		}
	}
}

func TestEvaluateDirectoryNameRecognizerWithStructuredCurrentNameWithoutAuthor(t *testing.T) {
	currentName := "【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)"

	captures, matched, err := evaluateDirectoryNameRecognizer("karita-manga-filename", "v1", currentName, currentName)
	if err != nil {
		t.Fatalf("evaluateDirectoryNameRecognizer failed: %v", err)
	}
	if !matched {
		t.Fatal("expected structured current name without explicit author to match karita-manga-filename@v1")
	}

	checks := map[string]string{
		"title":           "きょうこの日々",
		"scanlator_group": "CE家族社",
		"event_code":      "C82",
		"circle_name":     "とんずら道中",
		"original_work":   "東方Project",
	}
	for key, want := range checks {
		if got := captures[key]; got != want {
			t.Fatalf("expected %s=%q, got %q (captures=%#v)", key, want, got, captures)
		}
	}
}

func TestEvaluateDirectoryNameRecognizerWithScanlatorAuthorAliasPath(t *testing.T) {
	currentName := "【CE家族社】[黒澤pict(黒澤清崇)]制服触手8"
	relativePath := "comics/分散投资/【CE家族社】[黒澤pict(黒澤清崇)]制服触手8/" + currentName

	captures, matched, err := evaluateDirectoryNameRecognizer("karita-manga-filename", "v1", currentName, relativePath)
	if err != nil {
		t.Fatalf("evaluateDirectoryNameRecognizer failed: %v", err)
	}
	if !matched {
		t.Fatal("expected scanlator-author-alias path to match karita-manga-filename@v1")
	}

	checks := map[string]string{
		"title":           "制服触手8",
		"scanlator_group": "CE家族社",
		"author_alias":    "黒澤pict",
		"author_name":     "黒澤清崇",
	}
	for key, want := range checks {
		if got := captures[key]; got != want {
			t.Fatalf("expected %s=%q, got %q (captures=%#v)", key, want, got, captures)
		}
	}
}

func TestEvaluateDirectoryNameRecognizerStripsFixedTitleNoiseTags(t *testing.T) {
	currentName := "【CE家族社】[黒澤pict(黒澤清崇)]制服触手8 [中国翻訳] [DL版] [無修正]"
	relativePath := "comics/分散投资/【CE家族社】[黒澤pict(黒澤清崇)]制服触手8/" + currentName

	captures, matched, err := evaluateDirectoryNameRecognizer("karita-manga-filename", "v1", currentName, relativePath)
	if err != nil {
		t.Fatalf("evaluateDirectoryNameRecognizer failed: %v", err)
	}
	if !matched {
		t.Fatal("expected noisy title path to match karita-manga-filename@v1")
	}
	if got := captures["title"]; got != "制服触手8" {
		t.Fatalf("expected fixed noise tags to be stripped from title, got %q (captures=%#v)", got, captures)
	}
}
