package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStreamUsersSendsBearerTokenAndFollowsOpaqueOffset(t *testing.T) {
	const opaqueOffset = "next/page +token="
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got, want := r.URL.Path, "/workspaces/workspace-gid/users"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer test-token"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("limit"), "100"; got != want {
			t.Errorf("limit = %q, want %q", got, want)
		}

		switch requests {
		case 1:
			if got := r.URL.Query().Get("offset"); got != "" {
				t.Errorf("first page offset = %q, want empty", got)
			}
			fmt.Fprint(w, `{"data":[{"gid":"user-1","name":"Ada"}],"next_page":{"offset":"next/page +token="}}`)
		case 2:
			if got := r.URL.Query().Get("offset"); got != opaqueOffset {
				t.Errorf("second page offset = %q, want %q", got, opaqueOffset)
			}
			fmt.Fprint(w, `{"data":[{"gid":"user-2","name":"Lin"}],"next_page":null}`)
		default:
			t.Errorf("unexpected request number %d", requests)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := newAsanaClient("test-token", server.Client(), 120, 3)
	client.baseURL = server.URL

	var pages [][]User
	err := client.StreamUsers(context.Background(), "workspace-gid", func(page []User) error {
		pages = append(pages, page)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamUsers() error = %v", err)
	}

	if requests != 2 {
		t.Fatalf("request count = %d, want 2", requests)
	}
	want := [][]User{
		{{GID: "user-1", Name: "Ada"}},
		{{GID: "user-2", Name: "Lin"}},
	}
	if !reflect.DeepEqual(pages, want) {
		t.Errorf("pages = %#v, want %#v", pages, want)
	}
}

func TestStreamProjectsStopsWhenNextPageIsNull(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got, want := r.URL.Path, "/workspaces/workspace-gid/projects"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		fmt.Fprint(w, `{"data":[{"gid":"project-1","name":"Roadmap"}],"next_page":null}`)
	}))
	defer server.Close()

	client := newAsanaClient("test-token", server.Client(), 120, 3)
	client.baseURL = server.URL

	var received []Project
	err := client.StreamProjects(context.Background(), "workspace-gid", func(page []Project) error {
		received = append(received, page...)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamProjects() error = %v", err)
	}
	if requests != 1 {
		t.Errorf("request count = %d, want 1", requests)
	}
	want := []Project{{GID: "project-1", Name: "Roadmap"}}
	if !reflect.DeepEqual(received, want) {
		t.Errorf("projects = %#v, want %#v", received, want)
	}
}

func TestStreamUsersReturnsUsefulHTTPError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "invalid workspace", http.StatusBadRequest)
	}))
	defer server.Close()

	client := newAsanaClient("test-token", server.Client(), 120, 3)
	client.baseURL = server.URL

	err := client.StreamUsers(context.Background(), "workspace-gid", func([]User) error {
		return nil
	})
	if err == nil {
		t.Fatal("StreamUsers() error = nil, want HTTP error")
	}
	if got := err.Error(); !strings.Contains(got, "HTTP 400") || !strings.Contains(got, "invalid workspace") {
		t.Errorf("error = %q, want status and response body", got)
	}
	if requests != 1 {
		t.Errorf("request count = %d, want 1", requests)
	}
}

func TestStreamUsersHonorsRetryAfter(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"data":[{"gid":"user-1","name":"Ada"}],"next_page":null}`)
	}))
	defer server.Close()

	client := newAsanaClient("test-token", server.Client(), 600, 1)
	client.baseURL = server.URL
	start := time.Now()
	err := client.StreamUsers(context.Background(), "workspace-gid", func([]User) error {
		return nil
	})
	if err != nil {
		t.Fatalf("StreamUsers() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Errorf("retry completed after %v, want at least Retry-After 1s", elapsed)
	}
	if requests != 2 {
		t.Errorf("request count = %d, want 2", requests)
	}
}

func TestStreamUsersRespectsRetryLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "0")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	const maxRetries = 2
	client := newAsanaClient("test-token", server.Client(), 600, maxRetries)
	client.baseURL = server.URL
	err := client.StreamUsers(context.Background(), "workspace-gid", func([]User) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("StreamUsers() error = %v, want HTTP 429 error after retries", err)
	}
	if want := maxRetries + 1; requests != want {
		t.Errorf("request count = %d, want %d", requests, want)
	}
}

func TestStreamUsersRetriesServerErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"data":[{"gid":"user-1","name":"Ada"}],"next_page":null}`)
	}))
	defer server.Close()

	client := newAsanaClient("test-token", server.Client(), 600, 2)
	client.baseURL = server.URL
	err := client.StreamUsers(context.Background(), "workspace-gid", func([]User) error {
		return nil
	})
	if err != nil {
		t.Fatalf("StreamUsers() error = %v", err)
	}
	if requests != 2 {
		t.Errorf("request count = %d, want 2", requests)
	}
}

func TestStreamUsersRetryWaitIsCancellable(t *testing.T) {
	firstResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		close(firstResponse)
	}))
	defer server.Close()

	client := newAsanaClient("test-token", server.Client(), 600, 2)
	client.baseURL = server.URL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		result <- client.StreamUsers(ctx, "workspace-gid", func([]User) error {
			return nil
		})
	}()

	select {
	case <-firstResponse:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial rate-limit response")
	}
	cancel()

	select {
	case err := <-result:
		if !strings.Contains(err.Error(), context.Canceled.Error()) {
			t.Errorf("StreamUsers() error = %v, want context cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamUsers() did not return after context cancellation")
	}
}
