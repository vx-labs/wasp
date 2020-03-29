package stats

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	prometheusMetricsFactory promauto.Factory                = promauto.With(prometheus.DefaultRegisterer)
	gaugeVecs                map[string]*prometheus.GaugeVec = map[string]*prometheus.GaugeVec{
		"egressBytes": prometheusMetricsFactory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "wasp_egress_bytes",
			Help: "The total count of outgoing bytes.",
		}, []string{"protocol"}),
		"ingressBytes": prometheusMetricsFactory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "wasp_egress_bytes",
			Help: "The total count of incoming bytes.",
		}, []string{"protocol"}),
	}
	gauges map[string]prometheus.Gauge = map[string]prometheus.Gauge{
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
	histograms map[string]prometheus.Histogram = map[string]prometheus.Histogram{
		"publishLocalProcessingTime": prometheusMetricsFactory.NewHistogram(prometheus.HistogramOpts{
			Name:    "wasp_publish_packets_local_processing_time_milliseconds",
			Help:    "The time elapsed resolving recipients and distributing MQTT publish messages.",
			Buckets: []float64{0.0001, 0.0002, 0.0005, 0.0008, 1, 100, 5000},
		}),
		"publishRemoteProcessingTime": prometheusMetricsFactory.NewHistogram(prometheus.HistogramOpts{
			Name:    "wasp_publish_packets_remote_processing_time_milliseconds",
			Help:    "The time elapsed resolving recipients for MQTT publish messages.",
			Buckets: []float64{0.0001, 0.0002, 0.0005, 0.0008, 1, 100, 5000},
		}),
	}
	histogramVecs map[string]*prometheus.HistogramVec = map[string]*prometheus.HistogramVec{
		"sessionPacketHandling": prometheusMetricsFactory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "wasp_session_packets_processing_time_milliseconds",
			Help:    "The time elapsed handling session MQTT packets.",
			Buckets: []float64{0.001, 0.01, 0.1, 1, 50, 5000},
		}, []string{"packet_type"}),
	}
)

func HistogramVec(name string) *prometheus.HistogramVec {
	return histogramVecs[name]
}

func GaugeVec(name string) *prometheus.GaugeVec {
	return gaugeVecs[name]
}

func Histogram(name string) prometheus.Histogram {
	return histograms[name]
}
func Gauge(name string) prometheus.Gauge {
	return gauges[name]
}

func ListenAndServe(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), mux)
}
