// Package handlers is Programs' HTTP API: programs, classes, enrollments,
// events and attendance, the Canvas and Essential Education classes shown
// alongside them, and the reports. It was a module in core until it moved
// here.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-contracts/programs"
	"github.com/UnlockedLabs/unlocked-contracts/providerplatforms"
	"github.com/UnlockedLabs/unlocked-programs/internal/database"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// server is Programs' routes and what they use.
type server struct {
	store *database.DB
	// Client calls the providers' own APIs, such as Canvas's.
	Client            *http.Client
	providerPlatforms providerplatforms.ProviderPlatforms
	facilityFeatures  featureChecker
	reports           pdfRenderer
	// buckets cache the Canvas programs, classes and events. They are nil
	// without NATS (the tests).
	buckets map[string]nats.KeyValue
	// canvasInflight holds the Canvas cache keys being warmed, so each is
	// fetched once at a time.
	canvasInflight sync.Map
}

// featureChecker answers whether a feature is on at a facility, as
// features.Checker does.
type featureChecker interface {
	Enabled(ctx context.Context, facilityID uint, f features.Access) (bool, error)
}

// pdfRenderer renders a report as a PDF with the named template, such as
// "attendance_report".
type pdfRenderer interface {
	PDF(template string, report pdfReport) ([]byte, error)
}

// pdfReport is the table a PDF report shows.
type pdfReport struct {
	Title   string
	Rows    [][]string
	SubRows [][]string
	// Params are extra template parameters, such as which columns to show.
	// Set only those the template declares.
	Params map[string]string
	// Filters are the filters the report was run with, shown in its header.
	Filters []reportFilter
}

// reportFilter is one filter in a report's header, such as "Facility".
type reportFilter struct {
	Label string
	Value string
}

// clientTimeout caps each call Client makes, so a slow provider can't hold a
// request open.
const clientTimeout = 30 * time.Second

// Deps is what the routes use: the database, NATS for the Canvas caches, the
// services Programs asks, and Token, the shared service token core's calls
// carry.
type Deps struct {
	DB                *database.DB
	NATS              *nats.Conn
	ProviderPlatforms providerplatforms.ProviderPlatforms
	Features          featureChecker
	Reports           pdfRenderer
	Token             string
}

// NewHandler serves Programs' routes, each behind requireLogin and its access
// checks, its metrics on /api/metrics for Prometheus, and its answers to
// core's questions on /internal/v1/programs/, which the ingress never routes,
// to callers with deps.Token. A failure to set up the Canvas caches in NATS is
// an error. Without NATS (the tests) Canvas isn't cached.
func NewHandler(deps Deps, requireLogin func(http.Handler) http.Handler) (http.Handler, error) {
	var buckets map[string]nats.KeyValue
	if deps.NATS != nil {
		var err error
		if buckets, err = canvasBuckets(deps.NATS); err != nil {
			return nil, err
		}
	}
	srv := &server{
		store:             deps.DB,
		Client:            &http.Client{Timeout: clientTimeout},
		providerPlatforms: deps.ProviderPlatforms,
		facilityFeatures:  deps.Features,
		reports:           deps.Reports,
		buckets:           buckets,
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", srv.newRouter(requireLogin))
	mux.Handle("GET /api/metrics", promhttp.Handler())
	mux.Handle("/internal/v1/programs/", programs.NewHandler(newAPI(srv), deps.Token))
	return mux, nil
}

// WithUserContext is the store with r's context, so the audit hooks record
// the caller as the row's create or update user.
func (srv *server) WithUserContext(r *http.Request) *database.DB {
	return database.NewDB(srv.store.WithContext(r.Context()))
}

// The NATS key-value buckets for the Canvas caches.
const (
	CanvasPrograms = "canvas_programs"
	CanvasClasses  = "canvas_classes"
	CanvasEvents   = "canvas_events"
)

// canvasBuckets opens the Canvas cache buckets, creating them on first start.
func canvasBuckets(nc *nats.Conn) (map[string]nats.KeyValue, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("initialize JetStream: %w", err)
	}
	buckets := map[string]nats.KeyValue{}
	for _, bucket := range []string{CanvasPrograms, CanvasClasses, CanvasEvents} {
		kv, err := js.KeyValue(bucket)
		if err != nil {
			kv, err = js.CreateKeyValue(&nats.KeyValueConfig{Bucket: bucket, History: 1, TTL: 5 * time.Minute})
			if err != nil {
				return nil, fmt.Errorf("create bucket %s: %w", bucket, err)
			}
		}
		buckets[bucket] = kv
	}
	return buckets, nil
}
