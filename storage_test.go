package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONStorageSavesUsersAndProjectsSeparately(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "nested", "output")
	storage, err := newJSONStorage(outputDir)
	if err != nil {
		t.Fatalf("newJSONStorage() error = %v", err)
	}

	user := User{GID: "user-123", Name: "Ada", ResourceType: "user"}
	project := Project{
		GID:             "project-456",
		Name:            "Roadmap",
		ResourceSubtype: "default_project",
		ResourceType:    "project",
	}
	if err := storage.SaveUser(user); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	assertJSONFile(t, filepath.Join(outputDir, "users", "user-123.json"), user)
	assertJSONFile(t, filepath.Join(outputDir, "projects", "project-456.json"), project)
}

func TestJSONStorageRejectsInvalidGID(t *testing.T) {
	storage, err := newJSONStorage(t.TempDir())
	if err != nil {
		t.Fatalf("newJSONStorage() error = %v", err)
	}

	for _, gid := range []string{"", "..", "../outside", `..\outside`} {
		t.Run(gid, func(t *testing.T) {
			if err := storage.SaveUser(User{GID: gid, Name: "Ada"}); err == nil {
				t.Fatalf("SaveUser(GID=%q) error = nil, want error", gid)
			}
		})
	}
}

func assertJSONFile(t *testing.T, path string, want any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), "\n  \"") {
		t.Errorf("%s is not formatted as readable JSON:\n%s", path, data)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	wantData, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal expected JSON: %v", err)
	}
	var wantMap map[string]any
	if err := json.Unmarshal(wantData, &wantMap); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	if len(got) != len(wantMap) {
		t.Errorf("%s has %d fields, want %d", path, len(got), len(wantMap))
	}
	for key, value := range wantMap {
		if got[key] != value {
			t.Errorf("%s field %q = %v, want %v", path, key, got[key], value)
		}
	}
}
