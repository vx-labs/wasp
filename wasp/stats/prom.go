package stats

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	prometheusMetricsFactory promauto.Factory            = promauto.With(prometheus.DefaultRegisterer)
	gauges                   map[string]prometheus.Gauge = map[string]prometheus.Gauge{
		"subscriptionsCount": prometheusMetricsFactory.NewGauge(prometheus.GaugeOpts{
			Name: "wasp_subscriptions_count",
			Help: "The total number of MQTT subscriptions.",
		}),
		"retainedMessagesCount": prometheusMetricsFactory.NewGauge(prometheus.GaugeOpts{
			Name: "wasp_retained_messages_count",
			Help: "The total number of MQTT retained messages.",
		}),
		"sessionsCount": prometheusMetricsFactory.NewGauge(prometheus.GaugeOpts{
			Name: "wasp_sessions_count",
			Help: "The total number of MQTT sessions connected to this node.",
		}),
	}
	summaries map[string]prometheus.Summary = map[string]prometheus.Summary{
		"publishLocalProcessingTime": prometheusMetricsFactory.NewSummary(prometheus.SummaryOpts{
			Name: "wasp_publish_packets_local_processing_time_ms",
			Help: "The time elapsed resolving recipients and distributing MQTT publish messages.",
		}),
		"publishRemoteProcessingTime": prometheusMetricsFactory.NewSummary(prometheus.SummaryOpts{
			Name: "wasp_publish_packets_remote_processing_time_ms",
			Help: "The time elapsed resolving recipients for MQTT publish messages.",
		}),
	}
)

func Summary(name string) prometheus.Summary {
	return summaries[name]
}
func Gauge(name string) prometheus.Gauge {
	return gauges[name]
}

func ListenAndServe(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), mux)
}
