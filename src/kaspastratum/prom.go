package kaspastratum

import (
	"net/http"
	"sync"

	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var workerLabels = []string{
	"worker", "miner", "ip",
}

var shareCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_valid_share_counter",
	Help: "Number of shares found by worker over time",
}, workerLabels)

var staleCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_stale_share_counter",
	Help: "Number of stale shares found by worker over time",
}, workerLabels)

var invalidCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_invalid_share_counter",
	Help: "Number of stale shares found by worker over time",
}, workerLabels)

var blockCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_blocks_mined",
	Help: "Number of blocks mined over time",
}, workerLabels)

var disconnectCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_worker_disconnect_counter",
	Help: "Number of disconnects by worker",
}, workerLabels)

func commonLabels(worker *gostratum.StratumContext) prometheus.Labels {
	return prometheus.Labels{
		"worker": worker.WorkerName,
		"miner":  worker.RemoteApp,
		"ip":     worker.RemoteAddr,
	}
}

func RecordShareFound(worker *gostratum.StratumContext) {
	shareCounter.With(commonLabels(worker)).Inc()
}

func RecordStaleShare(worker *gostratum.StratumContext) {
	staleCounter.With(commonLabels(worker)).Inc()
}

func RecordInvalidShare(worker *gostratum.StratumContext) {
	invalidCounter.With(commonLabels(worker)).Inc()
}

func RecordBlockFound(worker *gostratum.StratumContext) {
	blockCounter.With(commonLabels(worker)).Inc()
}

func RecordDisconnect(worker *gostratum.StratumContext) {
	disconnectCounter.With(commonLabels(worker)).Inc()
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
