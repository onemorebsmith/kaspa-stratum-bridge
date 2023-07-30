package kaspastratum

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/kaspanet/kaspad/domain/consensus/utils/consensushashing"
	"github.com/kaspanet/kaspad/domain/consensus/utils/pow"
	"github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

const varDiffThreadSleep = 10

type WorkStats struct {
	BlocksFound        atomic.Int64
	SharesFound        atomic.Int64
	SharesDiff         atomic.Float64
	StaleShares        atomic.Int64
	InvalidShares      atomic.Int64
	WorkerName         string
	StartTime          time.Time
	LastShare          time.Time
	VarDiffStartTime   time.Time
	VarDiffSharesFound atomic.Int64
	VarDiffWindow      int
	MinDiff            atomic.Float64
}

type shareHandler struct {
	kaspa        *rpcclient.RPCClient
	stats        map[string]*WorkStats
	statsLock    sync.Mutex
	overall      WorkStats
	tipBlueScore uint64
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

		// TODO: not sure this is the best place, nor whether we shouldn't be
		// resetting on disconnect
		InitWorkerCounters(ctx)
	}

	sh.statsLock.Unlock()
	return stats
}

type submitInfo struct {
	block    *appmessage.RPCBlock
	state    *MiningState
	noncestr string
	nonceVal uint64
}

func validateSubmit(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) (*submitInfo, error) {
	if len(event.Params) < 3 {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return nil, fmt.Errorf("malformed event, expected at least 2 params")
	}
	jobIdStr, ok := event.Params[1].(string)
	if !ok {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return nil, fmt.Errorf("unexpected type for param 1: %+v", event.Params...)
	}
	jobId, err := strconv.ParseInt(jobIdStr, 10, 0)
	if err != nil {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return nil, errors.Wrap(err, "job id is not parsable as an number")
	}
	state := GetMiningState(ctx)
	block, exists := state.GetJob(int(jobId))
	if !exists {
		RecordWorkerError(ctx.WalletAddr, ErrMissingJob)
		return nil, fmt.Errorf("job does not exist. stale?")
	}
	noncestr, ok := event.Params[2].(string)
	if !ok {
		RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
		return nil, fmt.Errorf("unexpected type for param 2: %+v", event.Params...)
	}
	return &submitInfo{
		state:    state,
		block:    block,
		noncestr: strings.Replace(noncestr, "0x", "", 1),
	}, nil
}

var (
	ErrStaleShare = fmt.Errorf("stale share")
	ErrDupeShare  = fmt.Errorf("duplicate share")
)

// the max difference between tip blue score and job blue score that we'll accept
// anything greater than this is considered a stale
const workWindow = 8

func (sh *shareHandler) checkStales(ctx *gostratum.StratumContext, si *submitInfo) error {
	tip := sh.tipBlueScore
	if si.block.Header.BlueScore > tip {
		sh.tipBlueScore = si.block.Header.BlueScore
		return nil // can't be
	}
	if tip-si.block.Header.BlueScore > workWindow {
		RecordStaleShare(ctx)
		return errors.Wrapf(ErrStaleShare, "blueScore %d vs %d", si.block.Header.BlueScore, tip)
	}
	// TODO (bs): dupe share tracking
	return nil
}

func (sh *shareHandler) HandleSubmit(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) error {
	submitInfo, err := validateSubmit(ctx, event)
	if err != nil {
		return err
	}

	// add extranonce to noncestr if enabled and submitted nonce is shorter than
	// expected (16 - <extranonce length> characters)
	if ctx.Extranonce != "" {
		extranonce2Len := 16 - len(ctx.Extranonce)
		if len(submitInfo.noncestr) <= extranonce2Len {
			submitInfo.noncestr = ctx.Extranonce + fmt.Sprintf("%0*s", extranonce2Len, submitInfo.noncestr)
		}
	}

	//ctx.Logger.Debug(submitInfo.block.Header.BlueScore, " submit ", submitInfo.noncestr)
	state := GetMiningState(ctx)
	if state.useBigJob {
		submitInfo.nonceVal, err = strconv.ParseUint(submitInfo.noncestr, 16, 64)
		if err != nil {
			RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
			return errors.Wrap(err, "failed parsing noncestr")
		}
	} else {
		submitInfo.nonceVal, err = strconv.ParseUint(submitInfo.noncestr, 16, 64)
		if err != nil {
			RecordWorkerError(ctx.WalletAddr, ErrBadDataFromMiner)
			return errors.Wrap(err, "failed parsing noncestr")
		}
	}
	stats := sh.getCreateStats(ctx)
	// if err := sh.checkStales(ctx, submitInfo); err != nil {
	// 	if err == ErrDupeShare {
	// 		ctx.Logger.Info("dupe share "+submitInfo.noncestr, ctx.WorkerName, ctx.WalletAddr)
	// 		atomic.AddInt64(&stats.StaleShares, 1)
	// 		RecordDupeShare(ctx)
	// 		return ctx.ReplyDupeShare(event.Id)
	// 	} else if errors.Is(err, ErrStaleShare) {
	// 		ctx.Logger.Info(err.Error(), ctx.WorkerName, ctx.WalletAddr)
	// 		atomic.AddInt64(&stats.StaleShares, 1)
	// 		RecordStaleShare(ctx)
	// 		return ctx.ReplyStaleShare(event.Id)
	// 	}
	// 	// unknown error somehow
	// 	ctx.Logger.Error("unknown error during check stales: ", err.Error())
	// 	return ctx.ReplyBadShare(event.Id)
	// }

	converted, err := appmessage.RPCBlockToDomainBlock(submitInfo.block)
	if err != nil {
		return fmt.Errorf("failed to cast block to mutable block: %+v", err)
	}
	mutableHeader := converted.Header.ToMutable()
	mutableHeader.SetNonce(submitInfo.nonceVal)
	powState := pow.NewState(mutableHeader)
	powValue := powState.CalculateProofOfWorkValue()

	// The block hash must be less or equal than the claimed target.
	if powValue.Cmp(&powState.Target) <= 0 {
		if err := sh.submit(ctx, converted, submitInfo.nonceVal, event.Id); err != nil {
			return err
		}
	} else if powValue.Cmp(state.stratumDiff.targetValue) >= 0 {
		ctx.Logger.Warn("weak block")
		RecordWeakShare(ctx)
		return ctx.ReplyLowDiffShare(event.Id)
	}

	stats.SharesFound.Add(1)
	stats.VarDiffSharesFound.Add(1)
	stats.SharesDiff.Add(state.stratumDiff.hashValue)
	stats.LastShare = time.Now()
	sh.overall.SharesFound.Add(1)
	RecordShareFound(ctx, state.stratumDiff.hashValue)

	return ctx.Reply(gostratum.JsonRpcResponse{
		Id:     event.Id,
		Result: true,
	})
}

func (sh *shareHandler) submit(ctx *gostratum.StratumContext,
	block *externalapi.DomainBlock, nonce uint64, eventId any) error {
	mutable := block.Header.ToMutable()
	mutable.SetNonce(nonce)
	block = &externalapi.DomainBlock{
		Header:       mutable.ToImmutable(),
		Transactions: block.Transactions,
	}
	_, err := sh.kaspa.SubmitBlock(block)
	blockhash := consensushashing.BlockHash(block)
	// print after the submit to get it submitted faster
	ctx.Logger.Info(fmt.Sprintf("Submitted block %s", blockhash))

	if err != nil {
		// :'(
		if strings.Contains(err.Error(), "ErrDuplicateBlock") {
			ctx.Logger.Warn("block rejected, stale")
			// stale
			sh.getCreateStats(ctx).StaleShares.Add(1)
			sh.overall.StaleShares.Add(1)
			RecordStaleShare(ctx)
			return ctx.ReplyStaleShare(eventId)
		} else {
			ctx.Logger.Warn("block rejected, unknown issue (probably bad pow", zap.Error(err))
			sh.getCreateStats(ctx).InvalidShares.Add(1)
			sh.overall.InvalidShares.Add(1)
			RecordInvalidShare(ctx)
			return ctx.ReplyBadShare(eventId)
		}
	}

	// :)
	ctx.Logger.Info(fmt.Sprintf("block accepted %s", blockhash))
	stats := sh.getCreateStats(ctx)
	stats.BlocksFound.Add(1)
	sh.overall.BlocksFound.Add(1)
	RecordBlockFound(ctx, block.Header.Nonce(), block.Header.BlueScore(), blockhash.String())

	// nil return allows HandleSubmit to record share (blocks are shares too!)
	// and handle the response to the client
	return nil
}

func (sh *shareHandler) startStatsThread() error {
	start := time.Now()
	for {
		// console formatting is terrible. Good luck whever touches anything
		time.Sleep(10 * time.Second)

		// don't like locking entire stats struct - risk should be negligible
		// if mutex is ultimately needed, should move to one per client
		// sh.statsLock.Lock()

		str := "\n===============================================================================\n"
		str += "  worker name   |  avg hashrate  |   acc/stl/inv  |    blocks    |    uptime   \n"
		str += "-------------------------------------------------------------------------------\n"
		var lines []string
		totalRate := float64(0)
		for _, v := range sh.stats {
			// print stats
			rate := GetAverageHashrateGHs(v)
			totalRate += rate
			rateStr := stringifyHashrate(rate)
			ratioStr := fmt.Sprintf("%d/%d/%d", v.SharesFound.Load(), v.StaleShares.Load(), v.InvalidShares.Load())
			lines = append(lines, fmt.Sprintf(" %-15s| %14.14s | %14.14s | %12d | %11s",
				v.WorkerName, rateStr, ratioStr, v.BlocksFound.Load(), time.Since(v.StartTime).Round(time.Second)))
		}
		sort.Strings(lines)
		str += strings.Join(lines, "\n")
		rateStr := stringifyHashrate(totalRate)
		ratioStr := fmt.Sprintf("%d/%d/%d", sh.overall.SharesFound.Load(), sh.overall.StaleShares.Load(), sh.overall.InvalidShares.Load())
		str += "\n-------------------------------------------------------------------------------\n"
		str += fmt.Sprintf("                | %14.14s | %14.14s | %12d | %11s",
			rateStr, ratioStr, sh.overall.BlocksFound.Load(), time.Since(start).Round(time.Second))
		str += "\n========================================================== ks_bridge_" + version + " ===\n"
		// sh.statsLock.Unlock()
		log.Println(str)
	}
}

func GetAverageHashrateGHs(stats *WorkStats) float64 {
	return stats.SharesDiff.Load() / time.Since(stats.StartTime).Seconds()
}

func stringifyHashrate(ghs float64) string {
	unitStrings := [...]string{"M", "G", "T", "P", "E", "Z", "Y"}
	var unit string
	var hr float64

	if ghs < 1 {
		hr = ghs * 1000
		unit = unitStrings[0]
	} else if ghs < 1000 {
		hr = ghs
		unit = unitStrings[1]
	} else {
		for i, u := range unitStrings[2:] {
			hr = ghs / (float64(i) * 1000)
			if hr < 1000 {
				break
			}
			unit = u
		}
	}

	return fmt.Sprintf("%0.2f%sH/s", hr, unit)
}

func (sh *shareHandler) startVardiffThread(expectedShareRate uint, logStats bool) error {
	// 15 shares/min allows a ~95% confidence assumption of:
	//   < 100% variation after 1m
	//   < 50% variation after 3m
	//   < 25% variation after 10m
	//   < 15% variation after 30m
	//   < 10% variation after 1h
	//   < 5% variation after 4h
	var windows = [...]uint{1, 3, 10, 30, 60, 240, 0}
	var tolerances = [...]float64{1, 0.5, 0.25, 0.15, 0.1, 0.05, 0.05}

	for {
		time.Sleep(varDiffThreadSleep * time.Second)

		// don't like locking entire stats struct - risk should be negligible
		// if mutex is ultimately needed, should move to one per client
		// sh.statsLock.Lock()

		stats := "\n=== vardiff ===================================================================\n\n"
		stats += "  worker name  |    diff     |  window  |  elapsed   |    shares   |   rate    \n"
		stats += "-------------------------------------------------------------------------------\n"

		var statsLines []string
		var toleranceErrs []string

		for _, v := range sh.stats {
			if v.VarDiffStartTime.IsZero() {
				// no vardiff sent to client
				continue
			}

			worker := v.WorkerName
			diff := v.MinDiff.Load()
			shares := v.VarDiffSharesFound.Load()
			duration := time.Since(v.VarDiffStartTime).Minutes()
			shareRate := float64(shares) / duration
			shareRateRatio := shareRate / float64(expectedShareRate)
			window := windows[v.VarDiffWindow]
			tolerance := tolerances[v.VarDiffWindow]

			statsLines = append(statsLines, fmt.Sprintf(" %-14s| %11.2f | %8d | %10.2f | %11d | %9.2f", worker, diff, window, duration, shares, shareRate))

			// check final stage first, as this is where majority of time spent
			if window == 0 {
				if math.Abs(1-shareRateRatio) >= tolerance {
					// final stage submission rate OOB
					toleranceErrs = append(toleranceErrs, fmt.Sprintf("%s final share rate (%f) exceeded tolerance (+/- %d%%)", worker, shareRate, int(tolerance*100)))
					updateVarDiff(v, diff*shareRateRatio)
				}
				continue
			}

			// check all previously cleared windows
			i := 1
			for i < v.VarDiffWindow {
				if math.Abs(1-shareRateRatio) >= tolerances[i] {
					// breached tolerance of previously cleared window
					toleranceErrs = append(toleranceErrs, fmt.Sprintf("%s share rate (%f) exceeded tolerance (+/- %d%%) for %dm window", worker, shareRate, int(tolerances[i]*100), windows[i]))
					updateVarDiff(v, diff*shareRateRatio)
					break
				}
				i++
			}
			if i < v.VarDiffWindow {
				// should only happen if we broke previous loop
				continue
			}

			// check for current window max exception
			if float64(shares) >= float64(window*expectedShareRate)*(1+tolerance) {
				// submission rate > window max
				toleranceErrs = append(toleranceErrs, fmt.Sprintf("%s share rate (%f) exceeded upper tolerance (+ %d%%) for %dm window", worker, shareRate, int(tolerance*100), window))
				updateVarDiff(v, diff*shareRateRatio)
				continue
			}

			// check whether we've exceeded window length
			if duration >= float64(window) {
				// check for current window min exception
				if float64(shares) <= float64(window*expectedShareRate)*(1-tolerance) {
					// submission rate < window min
					toleranceErrs = append(toleranceErrs, fmt.Sprintf("%s share rate (%f) exceeded lower tolerance (- %d%%) for %dm window", worker, shareRate, int(tolerance*100), window))
					updateVarDiff(v, diff*math.Max(shareRateRatio, 0.1))
					continue
				}

				v.VarDiffWindow++
			}
		}
		sort.Strings(statsLines)
		stats += strings.Join(statsLines, "\n")
		stats += "\n\n========================================================== ks_bridge_" + version + " ===\n"
		stats += strings.Join(toleranceErrs, "\n")
		if logStats {
			log.Println(stats)
		}

		// sh.statsLock.Unlock()
	}
}

// update vardiff with new mindiff, reset counters, and disable tracker until
// client handler restarts it while sending diff on next block
func updateVarDiff(stats *WorkStats, minDiff float64) float64 {
	stats.VarDiffStartTime = time.Time{}
	stats.VarDiffWindow = 0
	previousMinDiff := stats.MinDiff.Load()
	stats.MinDiff.Store(math.Max(0.1, minDiff))
	return previousMinDiff
}

// (re)start vardiff tracker
func startVarDiff(stats *WorkStats) {
	if stats.VarDiffStartTime.IsZero() {
		stats.VarDiffSharesFound.Store(0)
		stats.VarDiffStartTime = time.Now()
	}
}

func (sh *shareHandler) startClientVardiff(ctx *gostratum.StratumContext) {
	stats := sh.getCreateStats(ctx)
	startVarDiff(stats)
}

func (sh *shareHandler) getClientVardiff(ctx *gostratum.StratumContext) float64 {
	stats := sh.getCreateStats(ctx)
	return stats.MinDiff.Load()
}

func (sh *shareHandler) setClientVardiff(ctx *gostratum.StratumContext, minDiff float64) float64 {
	stats := sh.getCreateStats(ctx)
	previousMinDiff := updateVarDiff(stats, minDiff)
	startVarDiff(stats)
	return previousMinDiff
}
