package kaspastratum

import (
	"fmt"
	"sync"

	"github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

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
	c.shareHandler.getCreateStats(ctx) // create the stats if they don't exist
}

func (c *clientListener) OnDisconnect(ctx *gostratum.StratumContext) {
	c.clientLock.Lock()
	delete(c.clients, ctx.RemoteAddr)
	c.clientLock.Unlock()
	RecordDisconnect(ctx.WorkerName)
}

func (c *clientListener) NewBlockAvailable(kapi *rpcclient.RPCClient) {
	for _, client := range c.clients {

		state := GetMiningState(client)
		if client.WalletAddr == "" {
			continue // not ready
		}

		template, err := kapi.GetBlockTemplate(client.WalletAddr, `"kaspa-stratum-bridge=["onemorebsmith"]`)
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed fetching new block template from kaspa: %s", err))
			return
		}
		state.bigDiff = CalculateTarget(uint64(template.Block.Header.Bits))
		header, err := SerializeBlockHeader(template.Block)
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed to serialize block header: %s", err))
			return
		}
		jobId := state.AddJob(template.Block)
		workNonce := GenerateJobHeader(header)

		if !state.initialized {
			state.initialized = true
			// first pass through send the difficulty since it's fixed
			if err := client.Send(stratumrpc.JsonRpcEvent{
				Version: "2.0",
				Method:  "mining.set_difficulty",
				Params:  []any{fixedDifficulty},
			}); err != nil {
				client.Logger.Error(errors.Wrap(err, "failed sending difficulty").Error())
			}
		}

		// // normal notify flow
		if err := client.Send(stratumrpc.JsonRpcEvent{
			Version: "2.0",
			Method:  "mining.notify",
			Id:      jobId,
			Params:  []any{fmt.Sprintf("%d", jobId), workNonce, template.Block.Header.Timestamp},
		}); err != nil {
			client.Logger.Error(errors.Wrap(err, "failed sending work packet").Error())
		}
	}
}
