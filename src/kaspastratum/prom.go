package kaspastratum

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var shareCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "valid_share_counter",
	Help: "Number of shares found by worker over time",
}, []string{
	"worker",
})
var staleCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "stale_share_counter",
	Help: "Number of stale shares found by worker over time",
}, []string{
	"worker",
})

var invalidCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "invalid_share_counter",
	Help: "Number of stale shares found by worker over time",
}, []string{
	"worker",
})

var blockCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "blocks_mined",
	Help: "Number of blocks mined over time",
}, []string{
	"worker",
})

var disconnectCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "worker_disconnect_counter",
	Help: "Number of disconnects by worker",
}, []string{
	"worker",
})

func RecordShareFound(worker string) {
	shareCounter.With(prometheus.Labels{
		"worker": worker,
	}).Inc()
}

func RecordStaleShare(worker string) {
	staleCounter.With(prometheus.Labels{
		"worker": worker,
	}).Inc()
}

func RecordInvalidShare(worker string) {
	invalidCounter.With(prometheus.Labels{
		"worker": worker,
	}).Inc()
}

func RecordBlockFound(worker string) {
	blockCounter.With(prometheus.Labels{
		"worker": worker,
	}).Inc()
}

func RecordDisconnect(worker string) {
	disconnectCounter.With(prometheus.Labels{
		"worker": worker,
	}).Inc()
}

var promInit sync.Once

func StartPromServer(log *zap.SugaredLogger, port string) {
	go func() { // prom http handler, separate from the main router
		promInit.Do(func() {
			logger := log.With(zap.String("server", "prometheus"))
			http.Handle("/metrics", promhttp.Handler())
			logger.Info("hosting prom stats on ", port, "/metrics")
			if err := http.ListenAndServe(port, nil); err != nil {
				logger.Error("error serving prom metrics", zap.Error(err))
			}
		})
	}()
}
