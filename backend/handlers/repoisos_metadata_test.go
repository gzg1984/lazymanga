package handlers

import (
	"testing"

	"lazymanga/models"
)

func TestPopulateRepoISOMetadataParsesMetadataJSON(t *testing.T) {
	row := &models.RepoISO{
		MetadataJSON: `{"item_kind":"archive","title":"J's 2","scanlator_group":"CE家族社"}`,
	}

	populateRepoISOMetadata(row)
	if row.Metadata == nil {
		t.Fatal("expected metadata to be parsed onto the API row")
	}
	if row.ItemKind != "archive" {
		t.Fatalf("expected item kind archive, got %q", row.ItemKind)
	}
	if row.Metadata["title"] != "J's 2" || row.Metadata["scanlator_group"] != "CE家族社" {
		t.Fatalf("unexpected parsed metadata: %#v", row.Metadata)
	}
}
