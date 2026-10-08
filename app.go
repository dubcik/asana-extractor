package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

type App struct {
	Config      Config
	HTTPClient  *http.Client
	AsanaClient *asanaClient
}

func newApp(cfg Config) *App {
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	return &App{
		Config:     cfg,
		HTTPClient: httpClient,
		AsanaClient: newAsanaClient(
			cfg.AsanaToken,
			httpClient,
			cfg.AsanaRequestsPerMinute,
			cfg.MaxRetries,
		),
	}
}

func (app *App) run(ctx context.Context) error {
	storage, err := newJSONStorage(app.Config.OutputDir)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(app.Config.PollInterval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			log.Printf("shutdown requested")
			return nil
		}
		if err := app.extract(ctx, storage); err != nil {
			if ctx.Err() != nil {
				log.Printf("extraction interrupted by shutdown")
				return nil
			}
			log.Printf("extraction failed: %v", err)
		}

		select {
		case <-ctx.Done():
			log.Printf("shutdown requested")
			return nil
		case <-ticker.C:
		}
	}
}

func (app *App) extract(ctx context.Context, storage *jsonStorage) error {
	log.Printf("extraction started for workspace %s", app.Config.AsanaWorkspaceGID)

	userCount := 0
	log.Printf("extracting users")
	if err := app.AsanaClient.StreamUsers(ctx, app.Config.AsanaWorkspaceGID, func(users []User) error {
		for _, user := range users {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := storage.SaveUser(user); err != nil {
				return err
			}
			userCount++
		}
		return nil
	}); err != nil {
		return err
	}
	log.Printf("saved %d users", userCount)

	projectCount := 0
	log.Printf("extracting projects")
	if err := app.AsanaClient.StreamProjects(ctx, app.Config.AsanaWorkspaceGID, func(projects []Project) error {
		for _, project := range projects {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := storage.SaveProject(project); err != nil {
				return err
			}
			projectCount++
		}
		return nil
	}); err != nil {
		return err
	}
	log.Printf("saved %d projects", projectCount)
	log.Printf("extraction finished: %d users, %d projects", userCount, projectCount)
	return nil
}
