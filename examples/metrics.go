package main

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/70data/golang-prometheus/prometheus"
	"github.com/70data/golang-prometheus/prometheus/promhttp"
)

var (
	responsesCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "module_responses",
		Help: "used to calculate qps, failure ratio",
	}, nil)
	responseLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "response_duration_milliseconds",
		Help:    "used to calculate app and api latency",
		Buckets: []float64{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 3000},
	}, nil)
	customLabels = []string{"app", "state"}
	connections  = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "module_connections",
		Help: "caculate connections by state",
	}, customLabels)
)

func init() {
	prometheus.MustRegister(responsesCounter)
	prometheus.MustRegister(responseLatency)
	prometheus.MustRegister(connections)
}

func main() {
	go func() {
		for {
			responsesCounter.WithLabelValues("gcs", "self", "/comment/post", "get", "200").Inc()
			time.Sleep(time.second)
		}
	}()
	go func() {
		for {
			responseLatency.WithLabelValues("gcs", "self", "/comment/post", "get", "200").Observe(rand.NormFloat64())
			time.Sleep(time.second)
		}
	}()
	go func() {
		for {
			connections.WithLabelValues("gcs", "reading").Set(50)
			time.Sleep(time.second)
		}
	}()
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2336", nil)
}
