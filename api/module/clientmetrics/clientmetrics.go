// Package clientmetrics provides a public API endpoint for recording
// client-side metrics. Frontend JavaScript can POST metric events (page
// views, button clicks, custom business metrics) and they are stored in
// the same column-based metrics engine used for system metrics.
//
// All client-submitted metric names are prefixed with "client_" to
// distinguish them from server-side metrics.
package clientmetrics

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/metrics"
)

const (
	// maxBatchSize is the maximum number of metrics per request.
	maxBatchSize = 20

	// maxNameLen is the maximum length of a metric name (before prefix).
	maxNameLen = 128

	// maxLabelCount is the maximum number of labels per metric.
	maxLabelCount = 10

	// maxLabelKeyLen is the maximum length of a label key.
	maxLabelKeyLen = 64

	// maxLabelValueLen is the maximum length of a label value.
	maxLabelValueLen = 128

	// clientPrefix is prepended to all client-submitted metric names.
	clientPrefix = "client_"
)

// namePattern validates metric names: lowercase alphanumeric and underscores.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// labelKeyPattern validates label keys: lowercase alphanumeric and underscores.
var labelKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// request is the JSON body for a single metric event.
type request struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels"`
}

// batchRequest is the JSON body for a batch of metric events.
type batchRequest struct {
	Metrics []request `json:"metrics"`
}

// Register mounts the client metrics endpoint on the given group.
//
//	POST /api/metrics — record one or more client-side metric events
func Register(api *http.Group, store *metrics.Store) {
	api.HandleFunc("POST /metrics", recordHandler(store))
}

func recordHandler(store *metrics.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit body to 64KB to prevent abuse.
		body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024+1))
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		if len(body) > 64*1024 {
			http.WriteError(w, http.StatusRequestEntityTooLarge, "request body too large: maximum 64KB")
			return
		}

		var raw json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// Accept both single metric and batch formats:
		//   {"metrics": [...]}   — batch
		//   {"name": "...", ...} — single (wrapped into batch internally)
		var items []request
		var batch batchRequest
		if err := json.Unmarshal(raw, &batch); err == nil && batch.Metrics != nil {
			items = batch.Metrics
		} else {
			var single request
			if err := json.Unmarshal(raw, &single); err != nil {
				http.WriteError(w, http.StatusBadRequest, "invalid JSON: expected {\"name\", \"value\"} or {\"metrics\": [...]}")
				return
			}
			items = []request{single}
		}

		if len(items) == 0 {
			http.WriteError(w, http.StatusBadRequest, "metrics array is empty")
			return
		}
		if len(items) > maxBatchSize {
			http.WriteError(w, http.StatusBadRequest, "too many metrics: maximum "+itoa(maxBatchSize)+" per request")
			return
		}

		// Validate and record each metric.
		for i, m := range items {
			if err := validateMetric(m); err != "" {
				http.WriteError(w, http.StatusBadRequest, "metrics["+itoa(i)+"]: "+err)
				return
			}

			// Build label pairs for Record.
			labels := make([]string, 0, len(m.Labels)*2)
			for k, v := range m.Labels {
				labels = append(labels, k, v)
			}

			store.Record(clientPrefix+m.Name, m.Value, labels...)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"recorded": len(items),
		})
	}
}

// validateMetric checks a single metric for validity. Returns an error
// message string, or empty string if valid.
func validateMetric(m request) string {
	if m.Name == "" {
		return "name is required"
	}
	if len(m.Name) > maxNameLen {
		return "name exceeds " + itoa(maxNameLen) + " characters"
	}
	if !namePattern.MatchString(m.Name) {
		return "name must be lowercase alphanumeric with underscores, starting with a letter"
	}
	if strings.HasPrefix(m.Name, clientPrefix) {
		return "name must not start with \"client_\" (prefix is added automatically)"
	}

	if len(m.Labels) > maxLabelCount {
		return "too many labels: maximum " + itoa(maxLabelCount)
	}
	for k, v := range m.Labels {
		if len(k) > maxLabelKeyLen {
			return "label key \"" + k + "\" exceeds " + itoa(maxLabelKeyLen) + " characters"
		}
		if !labelKeyPattern.MatchString(k) {
			return "label key \"" + k + "\" must be lowercase alphanumeric with underscores"
		}
		if len(v) > maxLabelValueLen {
			return "label value for \"" + k + "\" exceeds " + itoa(maxLabelValueLen) + " characters"
		}
	}

	return ""
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
