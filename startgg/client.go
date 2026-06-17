package startgg

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/shurcooL/graphql"
)

type SGGClient struct {
	Client *graphql.Client
}

// ClientOptions controls HTTP behavior for the start.gg GraphQL client.
type ClientOptions struct {
	Transport      http.RoundTripper
	MinInterval    time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

// DefaultClientOptions returns conservative defaults for start.gg's published
// rate limit: 80 requests / 60 seconds.
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		Transport:      http.DefaultTransport,
		MinInterval:    750 * time.Millisecond,
		MaxRetries:     2,
		RetryBaseDelay: time.Second,
		RetryMaxDelay:  8 * time.Second,
	}
}

// CreateClient returns a SGGClient containing an authenticated graphql Client
func CreateClient(token string) SGGClient {
	return CreateClientWithOptions(token, DefaultClientOptions())
}

// CreateClientWithOptions returns an authenticated client with explicit HTTP
// rate-limit/retry behavior.
func CreateClientWithOptions(token string, options ClientOptions) SGGClient {
	if options.Transport == nil {
		options.Transport = http.DefaultTransport
	}
	if options.RetryBaseDelay == 0 {
		options.RetryBaseDelay = time.Second
	}
	if options.RetryMaxDelay == 0 {
		options.RetryMaxDelay = 8 * time.Second
	}
	httpClient := &http.Client{
		Transport: &authTransport{
			Token:       token,
			Base:        options.Transport,
			Limiter:     newRequestLimiter(options.MinInterval),
			MaxRetries:  options.MaxRetries,
			BaseBackoff: options.RetryBaseDelay,
			MaxBackoff:  options.RetryMaxDelay,
		},
	}
	c := graphql.NewClient("https://api.start.gg/gql/alpha", httpClient)

	return SGGClient{Client: c}
}

// =====================
// Queries
// =====================

// GetTournamentIdFromSlug returns the tournament Id given the friendly url string
func (c SGGClient) GetTournamentIdFromSlug(slug string) int {
	var query struct {
		Tournament struct {
			Id int
		} `graphql:"tournament(slug: $slug)"`
	}
	variables := map[string]any{
		"slug": graphql.String(slug),
	}

	err := c.Client.Query(context.Background(), &query, variables)

	if err != nil {
		panic(err)
	}

	return query.Tournament.Id
}

// GetTop8 returns a list of the Top 8 sets in a given event
func (c SGGClient) GetTop8(eventId int) []Node {
	var query struct {
		Event struct {
			Name string
			Sets struct {
				Nodes []Node
			} `graphql:"sets(page: $page, perPage: $perPage, sortType: STANDARD)"`
		} `graphql:"event(id: $eventId)"`
	}
	variables := map[string]any{
		"eventId": graphql.ID(strconv.Itoa(eventId)),
		"page":    graphql.Int(1),
		// 11 possible sets, including GF reset
		"perPage": graphql.Int(11),
	}

	err := c.Client.Query(context.Background(), &query, variables)

	if err != nil {
		panic(err)
	}

	sets := make([]Node, 0, 11)
	// only include sets where loser places top 8
	for _, node := range query.Event.Sets.Nodes {
		if node.LPlacement < 8 {
			sets = append(sets, node)
		}
	}

	return sets
}

// authTransport adds authentication, serializes requests to stay under the
// published start.gg rate limit, and retries transient HTTP failures.
type authTransport struct {
	Token       string
	Base        http.RoundTripper
	Limiter     *requestLimiter
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := readRequestBody(req)
	if err != nil {
		return nil, err
	}

	var lastResp *http.Response
	var lastErr error
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepWithContext(req.Context(), t.backoff(attempt, lastResp)); err != nil {
				return lastResp, err
			}
		}
		if t.Limiter != nil {
			if err := t.Limiter.Wait(req.Context()); err != nil {
				return nil, err
			}
		}

		nextReq := req.Clone(req.Context())
		nextReq.Header = req.Header.Clone()
		nextReq.Header.Set("Authorization", "Bearer "+t.Token)
		if body != nil {
			nextReq.Body = io.NopCloser(bytes.NewReader(*body))
			nextReq.ContentLength = int64(len(*body))
		}

		resp, err := t.Base.RoundTrip(nextReq)
		if !shouldRetry(resp, err) || attempt == t.MaxRetries {
			return resp, err
		}
		lastResp = resp
		lastErr = err
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
	return lastResp, lastErr
}

func (t *authTransport) backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	delay := t.BaseBackoff << (attempt - 1)
	if delay > t.MaxBackoff {
		return t.MaxBackoff
	}
	return delay
}

func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
}

func readRequestBody(req *http.Request) (*[]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	return &body, nil
}

type requestLimiter struct {
	Interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func newRequestLimiter(interval time.Duration) *requestLimiter {
	if interval <= 0 {
		return nil
	}
	return &requestLimiter{Interval: interval}
}

func (l *requestLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	waitUntil := l.next
	if waitUntil.Before(now) {
		waitUntil = now
	}
	l.next = waitUntil.Add(l.Interval)
	l.mu.Unlock()

	return sleepWithContext(ctx, time.Until(waitUntil))
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
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
