package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/70data/prometheus-golang/prometheus"
	"github.com/70data/prometheus-golang/prometheus/promauto"
	"github.com/70data/prometheus-golang/prometheus/promhttp"
)

var (
	uniformDomain     = flag.Float64("uniform.domain", 0.0002, "The domain for the uniform distribution.")
	normDomain        = flag.Float64("normal.domain", 0.0002, "The domain for the normal distribution.")
	normMean          = flag.Float64("normal.mean", 0.00001, "The mean for the normal distribution.")
	oscillationPeriod = flag.Duration("oscillation-period", 10*time.Minute, "The duration of the rate oscillation period.")
)

var customLabels = []string{"app", "downstream"}

var (
	connections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "latency",
		Help: "caculate connections by state",
	}, customLabels)

	// Create a summary to track fictional interservice RPC latencies for three
	// distinct services with different latency distributions. These services are
	// differentiated via a "service" label.
	rpcDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "rpc_durations_seconds",
			Help:       "RPC latency distributions.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"service"},
	)
	// The same as above, but now as a histogram, and only for the normal
	// distribution. The buckets are targeted to the parameters of the
	// normal distribution, with 20 buckets centered on the mean, each
	// half-sigma wide.
	rpcDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "rpc_durations_histogram_seconds",
		Help:    "RPC latency distributions.",
		Buckets: prometheus.LinearBuckets(*normMean-5**normDomain, .5**normDomain, 20),
	})
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(rpcDurations)
	prometheus.MustRegister(rpcDurationsHistogram)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}

func decimal(value float64) float64 {
	return math.Trunc(value*1e2+0.5) * 1e-2
}

func recordMetrics() {

	go func() {
		for {
			tikvValue := decimal(float64(rand.Intn(10)))
			connections.WithLabelValues("tidb", "tikv").Set(tikvValue)
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		for {
			boltdbValue := decimal(float64(rand.Intn(10)))
			connections.WithLabelValues("tidb", "boltdb").Set(boltdbValue)
			time.Sleep(2 * time.Second)
		}
	}()

	start := time.Now()
	oscillationFactor := func() float64 {
		return 2 + math.Sin(math.Sin(2*math.Pi*float64(time.Since(start))/float64(*oscillationPeriod)))
	}

	// Periodically record some sample latencies for the three services.
	go func() {
		for {
			v := rand.Float64() * *uniformDomain
			rpcDurations.WithLabelValues("uniform").Observe(v)
			time.Sleep(time.Duration(100*oscillationFactor()) * time.Millisecond)
		}
	}()

	go func() {
		for {
			v := (rand.NormFloat64() * *normDomain) + *normMean
			rpcDurations.WithLabelValues("normal").Observe(v)
			// Demonstrate exemplar support with a dummy ID. This
			// would be something like a trace ID in a real
			// application.  Note the necessary type assertion. We
			// already know that rpcDurationsHistogram implements
			// the ExemplarObserver interface and thus don't need to
			// check the outcome of the type assertion.
			rpcDurationsHistogram.(prometheus.ExemplarObserver).ObserveWithExemplar(
				v, prometheus.Labels{"dummyID": fmt.Sprint(rand.Intn(100000))},
			)
			time.Sleep(time.Duration(75*oscillationFactor()) * time.Millisecond)
		}
	}()

	go func() {
		for {
			v := rand.ExpFloat64() / 1e6
			rpcDurations.WithLabelValues("exponential").Observe(v)
			time.Sleep(time.Duration(50*oscillationFactor()) * time.Millisecond)
		}
	}()

}

func main() {
	recordMetrics()
	var myCollector promhttp.DefaultCollector
	myCollector.ProcessCollector = false
	myCollector.GoCollector = false
	http.Handle("/metrics", promhttp.Handler(myCollector))

	http.Handle("/metrics1", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))

	_ = http.ListenAndServe(":2330", nil)
}
