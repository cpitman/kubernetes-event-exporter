package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type eventCollector struct {
	eventCountTotal *prometheus.Desc
	clientset       *kubernetes.Clientset
}

func newEventCollector() prometheus.Collector {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	c := &eventCollector{
		eventCountTotal: prometheus.NewDesc(
			"event_count_total",
			"The total number of events currently reported by kubernetes.",
			nil, nil,
		),
		clientset: clientset,
	}

	return c
}

// Describe returns all descriptions of the collector.
func (c *eventCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.eventCountTotal
}

func (c *eventCollector) Collect(ch chan<- prometheus.Metric) {
	events, err := c.clientset.CoreV1().Events("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	aggregate := make(map[[3]string]int32)
	for _, event := range events.Items {
		key := [3]string{event.Reason, event.Type, event.InvolvedObject.Kind}
		previousCount := aggregate[key]

		aggregate[key] = previousCount + event.Count
	}

	for key, value := range aggregate {
		ch <- prometheus.MustNewConstMetric(c.eventCountTotal, prometheus.GaugeValue, float64(value),
			"reason", key[0], "type", key[1], "involvedObjectKind", key[2])
	}
}

var (
	addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
)

var (
	coll = newEventCollector()
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(coll)
	// Add Go module build info.
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}

func main() {
	flag.Parse()

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))
	log.Fatal(http.ListenAndServe(*addr, nil))
}
