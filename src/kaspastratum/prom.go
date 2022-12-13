package kaspastratum

import (
	"fmt"
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

var shareDiffCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_valid_share_diff_counter",
	Help: "Total difficulty of shares found by worker over time",
}, workerLabels)

var invalidCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_invalid_share_counter",
	Help: "Number of stale shares found by worker over time",
}, append(workerLabels, "type"))

var blockCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ks_blocks_mined",
	Help: "Number of blocks mined over time",
}, workerLabels)

var blockGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "ks_mined_blocks_gauge",
	Help: "Gauge containing 1 unique instance per block mined",
}, append(workerLabels, "nonce", "bluescore", "hash"))

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

func RecordShareFound(worker *gostratum.StratumContext, shareDiff float64) {
	shareCounter.With(commonLabels(worker)).Inc()
	shareDiffCounter.With(commonLabels(worker)).Add(shareDiff)
}

func RecordStaleShare(worker *gostratum.StratumContext) {
	labels := commonLabels(worker)
	labels["type"] = "stale"
	invalidCounter.With(labels).Inc()
}

func RecordDupeShare(worker *gostratum.StratumContext) {
	labels := commonLabels(worker)
	labels["type"] = "duplicate"
	invalidCounter.With(labels).Inc()
}

func RecordInvalidShare(worker *gostratum.StratumContext) {
	labels := commonLabels(worker)
	labels["type"] = "invalid"
	invalidCounter.With(labels).Inc()
}

func RecordWeakShare(worker *gostratum.StratumContext) {
	labels := commonLabels(worker)
	labels["type"] = "weak"
	invalidCounter.With(labels).Inc()
}

func RecordBlockFound(worker *gostratum.StratumContext, nonce, bluescore uint64, hash string) {
	blockCounter.With(commonLabels(worker)).Inc()
	labels := commonLabels(worker)
	labels["nonce"] = fmt.Sprintf("%d", nonce)
	labels["bluescore"] = fmt.Sprintf("%d", bluescore)
	labels["hash"] = fmt.Sprintf("%d", hash)
	blockGauge.With(labels).Set(1)
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

func InitInvalidCounter(worker *gostratum.StratumContext, errorType string) {
	labels := commonLabels(worker)
	labels["type"] = errorType
	invalidCounter.With(labels).Add(0)
}

func InitWorkerCounters(worker *gostratum.StratumContext) {
  labels := commonLabels(worker)

	shareCounter.With(labels).Add(0)
	shareDiffCounter.With(labels).Add(0)

  errTypes := []string{"stale", "duplicate", "invalid", "weak"}
  for _, e := range errTypes {
    InitInvalidCounter(worker, e)
  }

	blockCounter.With(labels).Add(0)

	disconnectCounter.With(labels).Add(0)

	jobCounter.With(labels).Add(0)
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
