package stats

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func MilisecondsElapsed(from time.Time) float64 {
	return float64(time.Since(from)) / float64(time.Millisecond)
}

var (
	prometheusMetricsFactory promauto.Factory = promauto.With(prometheus.DefaultRegisterer)
)
var (
	EgressBytes = prometheusMetricsFactory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wasp_egress_bytes",
		Help: "The total count of outgoing bytes.",
	}, []string{"protocol"})

	IngressBytes = prometheusMetricsFactory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wasp_ingress_bytes",
		Help: "The total count of incoming bytes.",
	}, []string{"protocol"})

	SubscriptionsCount = prometheusMetricsFactory.NewGauge(prometheus.GaugeOpts{
		Name: "wasp_local_subscriptions",
		Help: "The total number of MQTT subscriptions.",
	})
	RetainedMessagesCount = prometheusMetricsFactory.NewGauge(prometheus.GaugeOpts{
		Name: "wasp_retained_messages",
		Help: "The total number of MQTT retained messages.",
	})
	SessionsCount = prometheusMetricsFactory.NewGauge(prometheus.GaugeOpts{
		Name: "wasp_connected_sessions",
		Help: "The total number of MQTT sessions connected to this node.",
	})
	PublishDistributionTime = prometheusMetricsFactory.NewHistogram(prometheus.HistogramOpts{
		Name:    "wasp_publish_distribution_time_milliseconds",
		Help:    "The time elapsed resolving recipient nodes for MQTT publish messages.",
		Buckets: []float64{0.0001, 0.0002, 0.0005, 0.0008, 1, 100, 5000},
	})
	PublishSchedulingTime = prometheusMetricsFactory.NewHistogram(prometheus.HistogramOpts{
		Name:    "wasp_publish_scheduling_time_milliseconds",
		Help:    "The time elapsed resolving local recipients for MQTT publish messages.",
		Buckets: []float64{0.0001, 0.0002, 0.0005, 0.0008, 1, 100, 5000},
	})
	PublishWritingTime = prometheusMetricsFactory.NewHistogram(prometheus.HistogramOpts{
		Name:    "wasp_publish_writing_time_milliseconds",
		Help:    "The time elapsed writing messages for local recipients.",
		Buckets: []float64{0.0001, 0.0002, 0.0005, 0.0008, 1, 100, 5000},
	})

	SessionPacketHandling = prometheusMetricsFactory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "wasp_packets_processing_time_milliseconds",
		Help:    "The time elapsed handling session MQTT packets.",
		Buckets: []float64{0.1, 1, 50, 5000},
	}, []string{"packet_type"})
)

func ListenAndServe(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), mux)
}
