package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	asanaBaseURL        = "https://app.asana.com/api/1.0"
	pageSize            = 100
	initialRetryBackoff = 100 * time.Millisecond
	maxRetryBackoff     = 5 * time.Second
)

type asanaClient struct {
	token                string
	httpClient           *http.Client
	baseURL              string
	requestsPerMinute    int
	maxRetries           int
	rateMu               sync.Mutex
	nextRequestAvailable time.Time
}

type asanaPage[T any] struct {
	Data     []T `json:"data"`
	NextPage *struct {
		Offset string `json:"offset"`
	} `json:"next_page"`
}

func newAsanaClient(token string, httpClient *http.Client, requestsPerMinute, maxRetries int) *asanaClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if requestsPerMinute <= 0 {
		requestsPerMinute = 120
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &asanaClient{
		token:             token,
		httpClient:        httpClient,
		baseURL:           asanaBaseURL,
		requestsPerMinute: requestsPerMinute,
		maxRetries:        maxRetries,
	}
}

func (c *asanaClient) StreamUsers(ctx context.Context, workspaceGID string, handlePage func([]User) error) error {
	return streamAsanaPages(ctx, c, workspaceGID, "users", handlePage)
}

func (c *asanaClient) StreamProjects(ctx context.Context, workspaceGID string, handlePage func([]Project) error) error {
	return streamAsanaPages(ctx, c, workspaceGID, "projects", handlePage)
}

func streamAsanaPages[T any](
	ctx context.Context,
	client *asanaClient,
	workspaceGID string,
	resource string,
	handlePage func([]T) error,
) error {
	if handlePage == nil {
		return fmt.Errorf("handle page callback is required")
	}

	endpoint := strings.TrimRight(client.baseURL, "/") +
		"/workspaces/" + url.PathEscape(workspaceGID) + "/" + resource
	nextOffset := ""
	pageNumber := 0

	for {
		page, err := getAsanaPage[T](ctx, client, endpoint, nextOffset)
		if err != nil {
			return fmt.Errorf("retrieve workspace %s %s: %w", workspaceGID, resource, err)
		}
		pageNumber++
		log.Printf("Asana %s page %d: received %d records", resource, pageNumber, len(page.Data))
		if err := handlePage(page.Data); err != nil {
			return fmt.Errorf("process workspace %s %s page: %w", workspaceGID, resource, err)
		}
		if page.NextPage == nil {
			return nil
		}
		if page.NextPage.Offset == "" {
			return fmt.Errorf("retrieve workspace %s %s: response next_page has an empty offset", workspaceGID, resource)
		}
		if page.NextPage.Offset == nextOffset {
			return fmt.Errorf("retrieve workspace %s %s: response next_page did not advance pagination", workspaceGID, resource)
		}

		nextOffset = page.NextPage.Offset
	}
}

func getAsanaPage[T any](ctx context.Context, client *asanaClient, endpoint, offset string) (*asanaPage[T], error) {
	query := url.Values{"limit": {fmt.Sprintf("%d", pageSize)}}
	if offset != "" {
		query.Set("offset", offset)
	}
	requestURL := endpoint + "?" + query.Encode()

	for attempt := 0; ; attempt++ {
		if err := client.waitForRequestSlot(ctx); err != nil {
			return nil, err
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+client.token)
		request.Header.Set("Accept", "application/json")

		response, err := client.httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}

		if attempt < client.maxRetries {
			var retryDelay time.Duration
			switch {
			case response.StatusCode == http.StatusTooManyRequests:
				retryDelay, err = retryAfterDuration(response.Header.Get("Retry-After"))
				if err != nil {
					response.Body.Close()
					return nil, err
				}
			case response.StatusCode >= http.StatusInternalServerError && response.StatusCode <= 599:
				retryDelay = retryBackoff(attempt)
			}

			if retryDelay > 0 || response.StatusCode == http.StatusTooManyRequests ||
				(response.StatusCode >= http.StatusInternalServerError && response.StatusCode <= 599) {
				log.Printf("Asana request returned HTTP %d; retry %d/%d after %s",
					response.StatusCode, attempt+1, client.maxRetries, retryDelay)
				response.Body.Close()
				if err := waitContext(ctx, retryDelay); err != nil {
					return nil, err
				}
				continue
			}
		}

		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			err := responseError(response)
			response.Body.Close()
			return nil, err
		}

		var page asanaPage[T]
		decodeErr := json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response: %w", decodeErr)
		}
		return &page, nil
	}
}

func retryBackoff(attempt int) time.Duration {
	if attempt >= 6 {
		return maxRetryBackoff
	}
	delay := initialRetryBackoff * time.Duration(1<<uint(attempt))
	if delay > maxRetryBackoff {
		return maxRetryBackoff
	}
	return delay
}

func (c *asanaClient) waitForRequestSlot(ctx context.Context) error {
	interval := time.Minute / time.Duration(c.requestsPerMinute)

	c.rateMu.Lock()
	now := time.Now()
	waitUntil := now
	if c.nextRequestAvailable.After(now) {
		waitUntil = c.nextRequestAvailable
	}
	c.nextRequestAvailable = waitUntil.Add(interval)
	c.rateMu.Unlock()

	wait := time.Until(waitUntil)
	if wait > 0 {
		log.Printf("Asana request pacing: waiting %s", wait)
	}
	return waitContext(ctx, wait)
}

func waitContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryAfterDuration(value string) (time.Duration, error) {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("Asana returned HTTP 429 with invalid Retry-After %q", value)
	}
	return time.Duration(seconds) * time.Second, nil
}

func responseError(response *http.Response) error {
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("Asana returned HTTP %d and response body could not be read: %w", response.StatusCode, readErr)
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("Asana returned HTTP %d %s", response.StatusCode, response.Status)
	}
	return fmt.Errorf("Asana returned HTTP %d %s: %s", response.StatusCode, response.Status, message)
}
