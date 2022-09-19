package kaspastratum

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var bigJobRegex = regexp.MustCompile(".*BzMiner.*")

type clientListener struct {
	logger       *zap.SugaredLogger
	shareHandler *shareHandler
	clientLock   sync.RWMutex
	clients      map[string]*gostratum.StratumContext
}

func newClientListener(logger *zap.SugaredLogger, shareHandler *shareHandler) *clientListener {
	return &clientListener{
		logger:       logger,
		clientLock:   sync.RWMutex{},
		shareHandler: shareHandler,
		clients:      make(map[string]*gostratum.StratumContext),
	}
}

func (c *clientListener) OnConnect(ctx *gostratum.StratumContext) {
	c.clientLock.Lock()
	c.clients[ctx.RemoteAddr] = ctx
	c.clientLock.Unlock()
	go func() {
		// hacky, but give time for the authorize to go through so we can use the worker name
		time.Sleep(5 * time.Second)
		c.shareHandler.getCreateStats(ctx) // create the stats if they don't exist
	}()
}

func (c *clientListener) OnDisconnect(ctx *gostratum.StratumContext) {
	ctx.Done()
	c.clientLock.Lock()
	delete(c.clients, ctx.RemoteAddr)
	c.clientLock.Unlock()
	RecordDisconnect(ctx)
}

func (c *clientListener) NewBlockAvailable(kapi *KaspaApi) {
	c.clientLock.Lock()
	addresses := make([]string, 0, len(c.clients))
	for _, c := range c.clients {
		if !c.Connected() {
			continue
		}
		go func(client *gostratum.StratumContext) {
			state := GetMiningState(client)
			if client.WalletAddr == "" {
				RecordWorkerError(client.WalletAddr, ErrFailedBlockFetch)
				return // not ready
			}

			template, err := kapi.GetBlockTemplate(client)
			if err != nil {
				RecordWorkerError(client.WalletAddr, ErrFailedBlockFetch)
				client.Logger.Error(fmt.Sprintf("failed fetching new block template from kaspa: %s", err))
				return
			}
			state.bigDiff = CalculateTarget(uint64(template.Block.Header.Bits))
			header, err := SerializeBlockHeader(template.Block)
			if err != nil {
				RecordWorkerError(client.WalletAddr, ErrBadDataFromMiner)
				client.Logger.Error(fmt.Sprintf("failed to serialize block header: %s", err))
				return
			}

			jobId := state.AddJob(template.Block)
			if !state.initialized {
				state.initialized = true
				state.useBigJob = bigJobRegex.MatchString(client.RemoteApp)
				// first pass through send the difficulty since it's fixed
				if err := client.Send(gostratum.JsonRpcEvent{
					Version: "2.0",
					Method:  "mining.set_difficulty",
					Params:  []any{fixedDifficulty},
				}); err != nil {
					RecordWorkerError(client.WalletAddr, ErrFailedSetDiff)
					client.Logger.Error(errors.Wrap(err, "failed sending difficulty").Error(), zap.Any("context", client))
					return
				}
			}

			jobParams := []any{fmt.Sprintf("%d", jobId)}
			if state.useBigJob {
				jobParams = append(jobParams, GenerateLargeJobParams(header, uint64(template.Block.Header.Timestamp)))
			} else {
				jobParams = append(jobParams, GenerateJobHeader(header))
				jobParams = append(jobParams, template.Block.Header.Timestamp)
			}

			// // normal notify flow
			if err := client.Send(gostratum.JsonRpcEvent{
				Version: "2.0",
				Method:  "mining.notify",
				Id:      jobId,
				Params:  jobParams,
			}); err != nil {
				if errors.Is(err, gostratum.ErrorDisconnected) {
					RecordWorkerError(client.WalletAddr, ErrDisconnected)
					return
				}
				RecordWorkerError(client.WalletAddr, ErrFailedSendWork)
				client.Logger.Error(errors.Wrap(err, "failed sending work packet").Error(),
					zap.Any("context", client))
			}

			RecordNewJob(client)
		}(c)
		addresses = append(addresses, c.WalletAddr)
	}
	c.clientLock.Unlock()

	if len(addresses) > 0 {
		go func() {
			balances, err := kapi.kaspad.GetBalancesByAddresses(addresses)
			if err != nil {
				c.logger.Warn("failed to get balances from kaspa, prom stats will be out of date", zap.Error(err))
				return
			}
			RecordBalances(balances)
		}()
	}
}
