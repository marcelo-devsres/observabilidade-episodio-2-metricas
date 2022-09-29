package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	appVersion string
	version    = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "version",
		Help: "Version information about this binary",
		ConstLabels: map[string]string{
			"version": appVersion,
		},
	})

	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests",
	}, []string{"code", "method"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of all HTTP requests",
		Buckets: []float64{0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.75, 1, 2.5},
	}, []string{"code", "handler", "method", "endpoint"})

	httpRequestSummaryDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "http_request_summary_duration_seconds",
		Help:       "Summary of HTTP requests duration",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"code", "handler", "method", "endpoint"})
)

func main() {
	version.Set(1)
	bind := ":8080"
	rand.Seed(time.Now().UnixNano())

	r := prometheus.NewRegistry()
	r.MustRegister(httpRequestsTotal)
	r.MustRegister(httpRequestDuration)
	r.MustRegister(httpRequestSummaryDuration)
	r.MustRegister(version)

	foundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Endpoint estável"))
	})

	fastRandomHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Endpoint com resposta aleatória (0-0.5s)"))
	})

	slowRandomHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Endpoint com resposta aleatória (0-2s)"))
	})

	notfoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	internalErrorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	foundChain := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "found", "endpoint": "/"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, foundHandler),
	)

	fastRandomChain := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "found", "endpoint": "/fast-random"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, fastRandomHandler),
	)

	slowRandomChain := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "found", "endpoint": "/slow-random"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, slowRandomHandler),
	)

	summaryChain := promhttp.InstrumentHandlerDuration(
		httpRequestSummaryDuration.MustCurryWith(prometheus.Labels{"handler": "found", "endpoint": "/summary"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, fastRandomChain),
	)

	mux := http.NewServeMux()
	mux.Handle("/", foundChain)
	mux.Handle("/fast-random", fastRandomChain)
	mux.Handle("/slow-random", slowRandomChain)
	mux.Handle("/summary", summaryChain)
	mux.Handle("/error", promhttp.InstrumentHandlerCounter(httpRequestsTotal, notfoundHandler))
	mux.Handle("/internal-error", promhttp.InstrumentHandlerCounter(httpRequestsTotal, internalErrorHandler))
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	// var srv *http.Server
	srv := &http.Server{Addr: bind, Handler: mux}

	log.Fatal(srv.ListenAndServe())
}
