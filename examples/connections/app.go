package main

import (
	"math"
	"math/rand"
	"net/http"
	"time"
	
	"github.com/70data/prometheus-golang/prometheus"
	"github.com/70data/prometheus-golang/prometheus/promhttp"
)

var (
	connections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "latency",
		Help: "caculate connections by state",
	}, []string{"app", "downstream"})
)

func init() {
	prometheus.MustRegister(connections)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}

func decimal(value float64) float64 {
	return math.Trunc(value*1e2+0.5) * 1e-2
}

func recordMetrics() {

	go func() {
		for {
			v := decimal(float64(rand.Intn(10)))
			connections.WithLabelValues("db", "db").Set(v)
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		for {
			v := decimal(float64(rand.Intn(10)))
			connections.WithLabelValues("kv", "kv").Set(v)
			time.Sleep(2 * time.Second)
		}
	}()

}

func main() {

	recordMetrics()

	http.Handle("/metrics", promhttp.Handler(
		promhttp.DefaultCollector{
			ProcessCollector: false,
			GoCollector:      false,
		},
	))
	
	_ = http.ListenAndServe(":2330", nil)

}
