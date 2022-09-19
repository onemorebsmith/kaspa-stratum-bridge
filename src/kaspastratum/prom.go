package kaspastratum

import (
	"net/http"
	"sync"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var workerLabels = []string{
	"worker", "miner", "wallet", "ip",
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

var jobCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_worker_job_counter",
	Help: "Number of jobs sent to the miner by worker over time",
}, workerLabels)

var balanceGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "ks_balance_by_wallet_gauge",
	Help: "Gauge representing the wallet balance for connected workers",
}, []string{"wallet"})

var errorByWallet = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_worker_errors",
	Help: "Gauge representing errors by worker",
}, []string{"wallet", "error"})

var estimatedNetworkHashrate = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "ks_estimated_network_hashrate_gauge",
	Help: "Gauge representing the estimated network hashrate",
})

var networkDifficulty = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "ks_network_difficulty_gauge",
	Help: "Gauge representing the network difficulty",
})

var networkBlockCount = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "ks_network_block_count",
	Help: "Gauge representing the network block count",
})

func commonLabels(worker *gostratum.StratumContext) prometheus.Labels {
	return prometheus.Labels{
		"worker": worker.WorkerName,
		"miner":  worker.RemoteApp,
		"wallet": worker.WalletAddr,
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

func RecordNewJob(worker *gostratum.StratumContext) {
	jobCounter.With(commonLabels(worker)).Inc()
}

func RecordNetworkStats(hashrate uint64, blockCount uint64, difficulty float64) {
	estimatedNetworkHashrate.Set(float64(hashrate))
	networkDifficulty.Set(difficulty)
	networkBlockCount.Set(float64(blockCount))
}

func RecordWorkerError(address string, shortError ErrorShortCodeT) {
	errorByWallet.With(prometheus.Labels{
		"wallet": address,
		"error":  string(shortError),
	}).Inc()
}

func RecordBalances(response *appmessage.GetBalancesByAddressesResponseMessage) {
	unique := map[string]struct{}{}
	for _, v := range response.Entries {
		// only set once per run
		if _, exists := unique[v.Address]; !exists {
			balanceGauge.With(prometheus.Labels{
				"wallet": v.Address,
			}).Set(float64(v.Balance) / 100000000)
			unique[v.Address] = struct{}{}
		}
	}
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
