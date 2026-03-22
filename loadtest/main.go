// Load test tool for the standalone Stanza application.
//
// Runs concurrent HTTP requests against a running server and reports
// throughput, latency percentiles, and error rates.
//
// Usage:
//
//	go run ./loadtest -base http://localhost:23710 -duration 30s -concurrency 50
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	endpoint string
	status   int
	latency  time.Duration
	err      error
	bytes    int64
}

type stats struct {
	mu        sync.Mutex
	total     int64
	success   int64
	failures  int64
	errors    int64
	latencies []time.Duration
}

func main() {
	baseURL := flag.String("base", "http://localhost:23710", "base URL of the standalone server")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	concurrency := flag.Int("concurrency", 50, "number of concurrent workers")
	scenario := flag.String("scenario", "all", "test scenario: health, auth, crud, dashboard, all")
	adminEmail := flag.String("admin-email", "admin@stanza.dev", "admin email for auth")
	adminPass := flag.String("admin-pass", "admin", "admin password for auth")
	flag.Parse()

	fmt.Printf("=== Stanza Load Test ===\n")
	fmt.Printf("Target:      %s\n", *baseURL)
	fmt.Printf("Duration:    %s\n", *duration)
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("Scenario:    %s\n\n", *scenario)

	// Login to get auth cookies
	cookies, err := login(*baseURL, *adminEmail, *adminPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Admin login: OK (%d cookies received)\n\n", len(cookies))

	scenarios := buildScenarios(*baseURL, cookies)

	var selected []testScenario
	switch *scenario {
	case "all":
		selected = scenarios
	default:
		for _, s := range scenarios {
			if s.name == *scenario {
				selected = append(selected, s)
			}
		}
		if len(selected) == 0 {
			fmt.Fprintf(os.Stderr, "Unknown scenario: %s\nAvailable: health, auth, crud, dashboard, all\n", *scenario)
			os.Exit(1)
		}
	}

	for _, sc := range selected {
		runScenario(sc, *concurrency, *duration)
	}

	fmt.Printf("\n=== Load Test Complete ===\n")
}

type testScenario struct {
	name     string
	requests []requestDef
}

type requestDef struct {
	name    string
	method  string
	path    string
	body    string
	cookies []*http.Cookie
}

func buildScenarios(base string, cookies []*http.Cookie) []testScenario {
	return []testScenario{
		{
			name: "health",
			requests: []requestDef{
				{name: "GET /health", method: "GET", path: base + "/api/health"},
			},
		},
		{
			name: "auth",
			requests: []requestDef{
				{name: "GET /admin/auth (status)", method: "GET", path: base + "/api/admin/auth/", cookies: cookies},
			},
		},
		{
			name: "dashboard",
			requests: []requestDef{
				{name: "GET /admin/dashboard", method: "GET", path: base + "/api/admin/dashboard", cookies: cookies},
				{name: "GET /admin/dashboard/charts", method: "GET", path: base + "/api/admin/dashboard/charts", cookies: cookies},
			},
		},
		{
			name: "crud",
			requests: []requestDef{
				{name: "GET /admin/users", method: "GET", path: base + "/api/admin/users", cookies: cookies},
				{name: "GET /admin/admins", method: "GET", path: base + "/api/admin/admins", cookies: cookies},
				{name: "GET /admin/settings", method: "GET", path: base + "/api/admin/settings", cookies: cookies},
				{name: "GET /admin/audit", method: "GET", path: base + "/api/admin/audit", cookies: cookies},
				{name: "GET /admin/sessions", method: "GET", path: base + "/api/admin/sessions", cookies: cookies},
				{name: "GET /admin/queue/stats", method: "GET", path: base + "/api/admin/queue/stats", cookies: cookies},
				{name: "GET /admin/cron", method: "GET", path: base + "/api/admin/cron", cookies: cookies},
			},
		},
	}
}

func runScenario(sc testScenario, concurrency int, duration time.Duration) {
	fmt.Printf("--- Scenario: %s (%d concurrent, %s) ---\n", sc.name, concurrency, duration)

	resultsByEndpoint := make(map[string]*stats)
	for _, r := range sc.requests {
		resultsByEndpoint[r.name] = &stats{}
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	var totalOps atomic.Int64

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					MaxIdleConnsPerHost: concurrency,
					MaxConnsPerHost:     concurrency,
				},
			}
			reqIdx := 0
			for {
				select {
				case <-stop:
					return
				default:
				}

				rd := sc.requests[reqIdx%len(sc.requests)]
				reqIdx++

				r := executeRequest(client, rd)
				totalOps.Add(1)

				s := resultsByEndpoint[rd.name]
				s.mu.Lock()
				s.total++
				if r.err != nil {
					s.errors++
				} else if r.status >= 200 && r.status < 300 {
					s.success++
				} else {
					s.failures++
				}
				s.latencies = append(s.latencies, r.latency)
				s.mu.Unlock()
			}
		}()
	}

	// Progress ticker
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Printf("  ... %d requests completed\n", totalOps.Load())
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(duration)
	close(stop)
	ticker.Stop()
	wg.Wait()

	// Print results
	for _, rd := range sc.requests {
		s := resultsByEndpoint[rd.name]
		printStats(rd.name, s, duration)
	}
	fmt.Println()
}

func executeRequest(client *http.Client, rd requestDef) result {
	var body io.Reader
	if rd.body != "" {
		body = bytes.NewBufferString(rd.body)
	}

	req, err := http.NewRequest(rd.method, rd.path, body)
	if err != nil {
		return result{endpoint: rd.name, err: err}
	}
	for _, c := range rd.cookies {
		req.AddCookie(c)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return result{endpoint: rd.name, latency: latency, err: err}
	}

	n, _ := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return result{
		endpoint: rd.name,
		status:   resp.StatusCode,
		latency:  latency,
		bytes:    n,
	}
}

func printStats(name string, s *stats, duration time.Duration) {
	if s.total == 0 {
		fmt.Printf("  %-35s  (no requests)\n", name)
		return
	}

	rps := float64(s.total) / duration.Seconds()

	sort.Slice(s.latencies, func(i, j int) bool {
		return s.latencies[i] < s.latencies[j]
	})

	p50 := percentile(s.latencies, 50)
	p95 := percentile(s.latencies, 95)
	p99 := percentile(s.latencies, 99)
	max := s.latencies[len(s.latencies)-1]

	errorRate := float64(s.errors+s.failures) / float64(s.total) * 100

	fmt.Printf("  %-35s  %6d reqs  %8.1f rps  p50=%6s  p95=%6s  p99=%6s  max=%6s  err=%.1f%%\n",
		name, s.total, rps,
		fmtDur(p50), fmtDur(p95), fmtDur(p99), fmtDur(max),
		errorRate,
	)
}

func percentile(sorted []time.Duration, pct int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := len(sorted) * pct / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func login(base, email, password string) ([]*http.Cookie, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	body := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	resp, err := client.Post(base+"/api/admin/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	u, _ := url.Parse(base)
	cookies := jar.Cookies(u)
	if len(cookies) == 0 {
		// Fall back to raw Set-Cookie headers (HttpOnly cookies may not be in jar)
		cookies = resp.Cookies()
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no cookies in login response")
	}

	return cookies, nil
}
