package importer

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceExportRejectsLegacyTopLevelVersionKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	writeTestFile(t, path, `{
  "format": "recuerd0.workspace_export",
  "version": 1,
  "workspace": {"id": 68},
  "memories": []
}`)

	_, err := scanWorkspaceExport(path)
	if err == nil || !strings.Contains(err.Error(), "workspace_export must be export v1") {
		t.Fatalf("legacy version key must be rejected with the v1 envelope error, got %v", err)
	}
}
