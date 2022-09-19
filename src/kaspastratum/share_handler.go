package kaspastratum

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/kaspanet/kaspad/domain/consensus/utils/pow"
	"github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/pkg/errors"
)

type WorkStats struct {
	SharesFound   int64
	StaleShares   int64
	InvalidShares int64
	WorkerName    string
	StartTime     time.Time
	LastShare     time.Time
}

type shareHandler struct {
	kaspa     *rpcclient.RPCClient
	stats     map[string]*WorkStats
	statsLock sync.Mutex
	overall   WorkStats
}

func newShareHandler(kaspa *rpcclient.RPCClient) *shareHandler {
	return &shareHandler{
		kaspa:     kaspa,
		stats:     map[string]*WorkStats{},
		statsLock: sync.Mutex{},
	}
}

func (sh *shareHandler) getCreateStats(ctx *gostratum.StratumContext) *WorkStats {
	sh.statsLock.Lock()
	var stats *WorkStats
	found := false
	if ctx.WorkerName != "" {
		stats, found = sh.stats[ctx.WorkerName]
	}
	if !found { // no worker name, check by remote address
		stats, found = sh.stats[ctx.RemoteAddr]
		if found {
			// no worker name, but remote addr is there
			// so replacet the remote addr with the worker names
			delete(sh.stats, ctx.RemoteAddr)
			stats.WorkerName = ctx.WorkerName
			sh.stats[ctx.WorkerName] = stats
		}
	}
	if !found { // legit doesn't exist, create it
		stats = &WorkStats{}
		stats.LastShare = time.Now()
		stats.WorkerName = ctx.RemoteAddr
		stats.StartTime = time.Now()
		sh.stats[ctx.RemoteAddr] = stats
	}

	sh.statsLock.Unlock()
	return stats
}

func (sh *shareHandler) HandleSubmit(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) error {
	if len(event.Params) < 2 {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return fmt.Errorf("malformed event, expected at least 2 params")
	}
	jobIdStr, ok := event.Params[1].(string)
	if !ok {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return fmt.Errorf("unexpected type for param 1: %+v", event.Params...)
	}
	jobId, err := strconv.ParseInt(jobIdStr, 10, 0)
	if err != nil {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return errors.Wrap(err, "job id is not parsable as an number")
	}
	state := GetMiningState(ctx)
	block, exists := state.GetJob(int(jobId))
	if !exists {
		RecordWorkerError(ctx.WalletAddr, ErrMissingJob)
		return fmt.Errorf("job does not exist. stale?")
	}
	noncestr, ok := event.Params[2].(string)
	if !ok {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return fmt.Errorf("unexpected type for param 2: %+v", event.Params...)
	}
	ctx.Logger.Info("submit " + noncestr)
	noncestr = strings.Replace(noncestr, "0x", "", 1)
	var nonce uint64
	if GetMiningState(ctx).useBigJob {
		nonce, err = strconv.ParseUint(noncestr, 16, 64)
		if err != nil {
			RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
			return errors.Wrap(err, "failed parsing noncestr")
		}
	} else {
		nonce, err = strconv.ParseUint(noncestr, 16, 64)
		if err != nil {
			RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
			return errors.Wrap(err, "failed parsing noncestr")
		}
	}

	converted, err := appmessage.RPCBlockToDomainBlock(block)
	if err != nil {
		return fmt.Errorf("failed to cast block to mutable block: %+v", err)
	}
	mutableHeader := converted.Header.ToMutable()
	mutableHeader.SetNonce(nonce)
	powState := pow.NewState(mutableHeader)
	powValue := powState.CalculateProofOfWorkValue()

	// The block hash must be less or equal than the claimed target.
	if powValue.Cmp(&powState.Target) <= 0 {
		ctx.Logger.Info("found block")
		return sh.submit(ctx, converted, nonce) // will reply
	}

	stats := sh.getCreateStats(ctx)
	atomic.AddInt64(&stats.SharesFound, 1)
	stats.LastShare = time.Now()
	RecordShareFound(ctx)

	return ctx.Reply(gostratum.JsonRpcResponse{
		Id:     event.Id,
		Result: true,
	})
}

func (sh *shareHandler) submit(ctx *gostratum.StratumContext,
	block *externalapi.DomainBlock, nonce uint64) error {
	ctx.Logger.Info("submitting block to kaspad")
	ctx.Logger.Info(fmt.Sprintf("Submitting nonce: %d", nonce))
	mutable := block.Header.ToMutable()
	mutable.SetNonce(nonce)
	_, err := sh.kaspa.SubmitBlock(&externalapi.DomainBlock{
		Header:       mutable.ToImmutable(),
		Transactions: block.Transactions,
	})

	if err != nil {
		// :'(
		if strings.Contains(err.Error(), "ErrDuplicateBlock") {
			ctx.Logger.Warn("block rejected, stale")
			// stale
			atomic.AddInt64(&sh.getCreateStats(ctx).StaleShares, 1)
			atomic.AddInt64(&sh.overall.StaleShares, 1)
			RecordStaleShare(ctx)
			return ctx.Reply(gostratum.JsonRpcResponse{
				Result: []any{21, "Job not found", nil},
			})
		} else {
			ctx.Logger.Warn("block rejected, unknown issue (probably bad pow")
			atomic.AddInt64(&sh.getCreateStats(ctx).InvalidShares, 1)
			atomic.AddInt64(&sh.overall.InvalidShares, 1)
			RecordInvalidShare(ctx)
			return ctx.Reply(gostratum.JsonRpcResponse{
				Result: []any{20, "Unknown problem", nil},
			})
		}
	}

	// :)
	ctx.Logger.Info("block accepted")
	stats := sh.getCreateStats(ctx)
	stats.LastShare = time.Now()
	atomic.AddInt64(&stats.SharesFound, 1)
	atomic.AddInt64(&sh.overall.SharesFound, 1)
	RecordBlockFound(ctx)
	return ctx.Reply(gostratum.JsonRpcResponse{
		Result: true,
	})
}

func (sh *shareHandler) startStatsThread() error {
	start := time.Now()
	for {
		time.Sleep(10 * time.Second)
		sh.statsLock.Lock()
		str := "\n========================================================\n"
		str += fmt.Sprintf("uptime %s | mined %d | stales %d | reject %d \n",
			time.Since(start).Round(time.Second), sh.overall.SharesFound,
			sh.overall.StaleShares, sh.overall.InvalidShares)
		str += "--------------------------------------------------------\n"
		str += "worker\t| avg hashrate\t| shares\t| uptime\n"
		str += "--------------------------------------------------------\n"
		var lines []string
		totalRate := float64(0)
		for _, v := range sh.stats {
			// if len(v.WorkerName) == 0 || time.Since(v.LastShare) > time.Minute*5 {
			// 	continue
			// }
			rate := GetAverageHashrateGHz(v)
			totalRate += rate
			lines = append(lines, fmt.Sprintf("%s\t| %0.2fGH/s\t| %d\t| %s",
				v.WorkerName, rate, v.SharesFound, time.Since(v.StartTime).Round(time.Second)))
		}
		sort.Strings(lines)
		str += strings.Join(lines, "\n")
		str += "\n--------------------------------------------------------\n"
		str += fmt.Sprintf("total\t| %0.2fGH/s", totalRate)
		str += "\n======================================== ks_bridge_v.1.1\n"
		sh.statsLock.Unlock()
		log.Println(str)
	}
}

func GetAverageHashrateGHz(stats *WorkStats) float64 {
	return (float64(stats.SharesFound) * shareValue) / time.Since(stats.StartTime).Seconds()
}
