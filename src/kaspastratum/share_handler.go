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

	bufferLock    int32
	currentTarget *targetBuffer
}

type targetBuffer struct {
	DAAScore    uint64
	Lock        int32
	FoundShares map[uint64]struct{}
}

func (tb *targetBuffer) addShare(nonce uint64) bool {
	for {
		if atomic.CompareAndSwapInt32(&tb.Lock, 0, 1) {
			break
		}
	}
	exists := false
	if _, exists = tb.FoundShares[nonce]; !exists {
		tb.FoundShares[nonce] = struct{}{}
	}
	atomic.StoreInt32(&tb.Lock, 0)
	return !exists
}

func (s *shareHandler) getCreateBuffer(DAAScore uint64) *targetBuffer {
	for {
		if atomic.CompareAndSwapInt32(&s.bufferLock, 0, 1) {
			break
		}
	}
	ret := s.currentTarget
	if s.currentTarget.DAAScore < DAAScore {
		ret = &targetBuffer{
			DAAScore:    DAAScore,
			FoundShares: make(map[uint64]struct{}, 16),
		}
		s.currentTarget = ret
	}
	atomic.StoreInt32(&s.bufferLock, 0)
	return ret
}

func newShareHandler(kaspa *rpcclient.RPCClient) *shareHandler {
	return &shareHandler{
		kaspa:         kaspa,
		stats:         map[string]*WorkStats{},
		statsLock:     sync.Mutex{},
		currentTarget: &targetBuffer{},
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

type submitInfo struct {
	block    *appmessage.RPCBlock
	state    *MiningState
	noncestr string
	nonceVal uint64
}

func validateSubmit(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) (*submitInfo, error) {
	if len(event.Params) < 2 {
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

func (sh *shareHandler) checkStales(ctx *gostratum.StratumContext, si *submitInfo) error {
	buffer := sh.getCreateBuffer(si.block.Header.DAAScore)
	if si.block.Header.DAAScore < buffer.DAAScore {
		RecordStaleShare(ctx)
		return errors.Wrapf(ErrStaleShare, "daa %d vs %d", si.block.Header.DAAScore, buffer.DAAScore)
	} else if !buffer.addShare(si.nonceVal) {
		RecordDupeShare(ctx)
		return ErrDupeShare
	}
	return nil
}

func (sh *shareHandler) HandleSubmit(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) error {
	submitInfo, err := validateSubmit(ctx, event)
	if err != nil {
		return err
	}

	ctx.Logger.Debug(submitInfo.block.Header.DAAScore, " submit ", submitInfo.noncestr)
	if GetMiningState(ctx).useBigJob {
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
	if err := sh.checkStales(ctx, submitInfo); err != nil {
		if err == ErrDupeShare {
			ctx.Logger.Info("dupe share " + submitInfo.noncestr)
			atomic.AddInt64(&stats.StaleShares, 1)
			RecordDupeShare(ctx)
			return ctx.ReplyDupeShare(event.Id)
		} else /*err == ErrStaleShare*/ {
			ctx.Logger.Info(err.Error())
			atomic.AddInt64(&stats.StaleShares, 1)
			RecordStaleShare(ctx)
			// just record for now
			//return ctx.ReplyStaleShare(event.Id)
		}
	}

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
		return sh.submit(ctx, converted, submitInfo.nonceVal, event.Id)
	} else if powValue.Cmp(fixedDifficultyBI) >= 0 {
		ctx.Logger.Warn("weak block")
		RecordWeakShare(ctx)
		return ctx.ReplyLowDiffShare(event.Id)
	}

	atomic.AddInt64(&stats.SharesFound, 1)
	stats.LastShare = time.Now()
	RecordShareFound(ctx)

	return ctx.Reply(gostratum.JsonRpcResponse{
		Id:     event.Id,
		Result: true,
	})
}

func (sh *shareHandler) submit(ctx *gostratum.StratumContext,
	block *externalapi.DomainBlock, nonce uint64, eventId any) error {
	mutable := block.Header.ToMutable()
	mutable.SetNonce(nonce)
	_, err := sh.kaspa.SubmitBlock(&externalapi.DomainBlock{
		Header:       mutable.ToImmutable(),
		Transactions: block.Transactions,
	})
	// print after the submit to get it submitted faster
	ctx.Logger.Info("submitted block to kaspad")
	ctx.Logger.Info(fmt.Sprintf("Submitted nonce: %d", nonce))

	if err != nil {
		// :'(
		if strings.Contains(err.Error(), "ErrDuplicateBlock") {
			ctx.Logger.Warn("block rejected, stale")
			// stale
			atomic.AddInt64(&sh.getCreateStats(ctx).StaleShares, 1)
			atomic.AddInt64(&sh.overall.StaleShares, 1)
			RecordStaleShare(ctx)
			return ctx.ReplyStaleShare(eventId)
		} else {
			ctx.Logger.Warn("block rejected, unknown issue (probably bad pow")
			atomic.AddInt64(&sh.getCreateStats(ctx).InvalidShares, 1)
			atomic.AddInt64(&sh.overall.InvalidShares, 1)
			RecordInvalidShare(ctx)
			return ctx.ReplyBadShare(eventId)
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
		var stales, invalids int64
		for _, v := range sh.stats {
			stales += v.StaleShares
			invalids += v.InvalidShares
		}

		str := "\n========================================================\n"
		str += fmt.Sprintf("uptime %s | mined %d | stales %d | invalid %d \n",
			time.Since(start).Round(time.Second), sh.overall.SharesFound, stales, invalids)
		str += "--------------------------------------------------------\n"
		str += "worker\t| avg hashrate\t| a/s/i\t| uptime\n"
		str += "--------------------------------------------------------\n"
		var lines []string
		totalRate := float64(0)
		for _, v := range sh.stats {
			// if len(v.WorkerName) == 0 || time.Since(v.LastShare) > time.Minute*5 {
			// 	continue
			// }
			rate := GetAverageHashrateGHz(v)
			totalRate += rate
			lines = append(lines, fmt.Sprintf("%s\t| %0.2fGH/s\t| %d/%d/%d\t| %s",
				v.WorkerName, rate, v.SharesFound, v.StaleShares, v.InvalidShares,
				time.Since(v.StartTime).Round(time.Second)))
		}
		sort.Strings(lines)
		str += strings.Join(lines, "\n")
		str += "\n--------------------------------------------------------\n"
		str += fmt.Sprintf("total\t| %0.2fGH/s", totalRate)
		str += "\n======================================== ks_bridge_" + version + "\n"
		sh.statsLock.Unlock()
		log.Println(str)
	}
}

func GetAverageHashrateGHz(stats *WorkStats) float64 {
	return (float64(stats.SharesFound) * shareValue) / time.Since(stats.StartTime).Seconds()
}
