// Package adminmetrics provides the metrics administration endpoints. It
// exposes metric names, label values, time-series queries, and store
// statistics from the column-based metrics engine.
package adminmetrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/metrics"
)

// Register mounts the metrics admin routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/metrics/names  — list all metric names
//	GET /api/admin/metrics/labels — label values for a metric + key
//	GET /api/admin/metrics/query  — query time-series data
//	GET /api/admin/metrics/stats  — store statistics
func Register(admin *http.Group, store *metrics.Store) {
	admin.HandleFunc("GET /metrics/names", namesHandler(store))
	admin.HandleFunc("GET /metrics/labels", labelsHandler(store))
	admin.HandleFunc("GET /metrics/query", queryHandler(store))
	admin.HandleFunc("GET /metrics/stats", statsHandler(store))
}

func namesHandler(store *metrics.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"names": store.Names(),
		})
	}
}

func labelsHandler(store *metrics.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.WriteError(w, http.StatusBadRequest, "name parameter is required")
			return
		}
		key := r.URL.Query().Get("key")
		if key == "" {
			http.WriteError(w, http.StatusBadRequest, "key parameter is required")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"values": store.LabelValues(name, key),
		})
	}
}

func queryHandler(store *metrics.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		name := q.Get("name")
		if name == "" {
			http.WriteError(w, http.StatusBadRequest, "name parameter is required")
			return
		}

		startStr := q.Get("start")
		if startStr == "" {
			http.WriteError(w, http.StatusBadRequest, "start parameter is required")
			return
		}
		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid start: expected RFC3339")
			return
		}

		endStr := q.Get("end")
		if endStr == "" {
			http.WriteError(w, http.StatusBadRequest, "end parameter is required")
			return
		}
		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid end: expected RFC3339")
			return
		}

		step := time.Minute
		if stepStr := q.Get("step"); stepStr != "" {
			step, err = time.ParseDuration(stepStr)
			if err != nil {
				http.WriteError(w, http.StatusBadRequest, "invalid step: expected Go duration (e.g. 1m, 5m, 1h)")
				return
			}
		}

		fn := metrics.Avg
		if fnStr := q.Get("fn"); fnStr != "" {
			fn, err = parseAggFn(fnStr)
			if err != nil {
				http.WriteError(w, http.StatusBadRequest, "invalid fn: expected sum, avg, min, max, count, or last")
				return
			}
		}

		mq := metrics.Query{
			Name:  name,
			Start: start,
			End:   end,
			Step:  step,
			Fn:    fn,
		}

		// Parse label filters: "method=GET,status=200"
		if labelsStr := q.Get("labels"); labelsStr != "" {
			mq.Labels = make(map[string]string)
			for _, pair := range strings.Split(labelsStr, ",") {
				k, v, ok := strings.Cut(pair, "=")
				if ok {
					mq.Labels[strings.TrimSpace(k)] = strings.TrimSpace(v)
				}
			}
		}

		result, err := store.Query(mq)
		if err != nil {
			http.WriteServerError(w, r, "metrics query failed", err)
			return
		}

		// Map framework types to JSON-friendly response.
		type pointJSON struct {
			T int64   `json:"t"`
			V float64 `json:"v"`
		}
		type seriesJSON struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
			Points []pointJSON       `json:"points"`
		}
		series := make([]seriesJSON, 0, len(result.Series))
		for _, sd := range result.Series {
			pts := make([]pointJSON, len(sd.Points))
			for i, p := range sd.Points {
				pts[i] = pointJSON{T: p.T, V: p.V}
			}
			series = append(series, seriesJSON{
				Name:   sd.Name,
				Labels: sd.Labels,
				Points: pts,
			})
		}
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"series": series,
		})
	}
}

func statsHandler(store *metrics.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		s := store.Stats()
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"series_count":    s.SeriesCount,
			"partition_count": s.PartitionCount,
			"disk_bytes":      s.DiskBytes,
			"oldest_date":     s.OldestDate,
			"newest_date":     s.NewestDate,
		})
	}
}

func parseAggFn(s string) (metrics.AggFn, error) {
	switch strings.ToLower(s) {
	case "sum":
		return metrics.Sum, nil
	case "avg":
		return metrics.Avg, nil
	case "min":
		return metrics.Min, nil
	case "max":
		return metrics.Max, nil
	case "count":
		return metrics.Count, nil
	case "last":
		return metrics.Last, nil
	default:
		return 0, fmt.Errorf("unknown aggregation function: %s", s)
	}
}
