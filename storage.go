package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type jsonStorage struct {
	usersDir    string
	projectsDir string
}

func newJSONStorage(outputDir string) (*jsonStorage, error) {
	storage := &jsonStorage{
		usersDir:    filepath.Join(outputDir, "users"),
		projectsDir: filepath.Join(outputDir, "projects"),
	}
	if err := os.MkdirAll(storage.usersDir, 0755); err != nil {
		return nil, fmt.Errorf("create users output directory: %w", err)
	}
	if err := os.MkdirAll(storage.projectsDir, 0755); err != nil {
		return nil, fmt.Errorf("create projects output directory: %w", err)
	}
	return storage, nil
}

func (s *jsonStorage) SaveUser(user User) error {
	return writeJSONRecord(s.usersDir, user.GID, user)
}

func (s *jsonStorage) SaveProject(project Project) error {
	return writeJSONRecord(s.projectsDir, project.GID, project)
}

func writeJSONRecord(directory, gid string, record any) error {
	if gid == "" || gid == "." || gid == ".." || strings.ContainsAny(gid, `/\`) {
		return fmt.Errorf("invalid entity GID %q", gid)
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("format JSON for GID %s: %w", gid, err)
	}
	data = append(data, '\n')

	path := filepath.Join(directory, gid+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
