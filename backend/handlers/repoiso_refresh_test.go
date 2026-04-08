package handlers

import (
	"testing"

	"lazymanga/models"
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
