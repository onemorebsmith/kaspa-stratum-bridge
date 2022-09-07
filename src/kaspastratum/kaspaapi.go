package kaspastratum

import (
	"context"
	"time"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type KaspaApi struct {
	address string
	logger  *zap.SugaredLogger
	kaspad  *rpcclient.RPCClient
}

func NewKaspaAPI(address string, logger *zap.SugaredLogger) (*KaspaApi, error) {
	client, err := rpcclient.NewRPCClient(address)
	if err != nil {
		return nil, err
	}

	return &KaspaApi{
		address: address,
		logger:  logger.With(zap.String("component", "kaspaapi:"+address)),
		kaspad:  client,
	}, nil
}

func (ks *KaspaApi) Start(ctx context.Context, blockCb func()) {
	ks.waitForSync()
	go ks.startBlockTemplateListener(ctx, blockCb)

}

func (s *KaspaApi) waitForSync() error {
	for {
		s.logger.Info("checking kaspad sync state")
		clientInfo, err := s.kaspad.GetInfo()
		if err != nil {
			return errors.Wrapf(err, "error fetching server info from kaspad @ %s", s.address)
		}
		if clientInfo.IsSynced {
			break
		}
		s.logger.Warn("Kaspa is not synced, waiting for sync before starting bridge")
		time.Sleep(5 * time.Second)
	}
	s.logger.Info("kaspad synced, starting server")
	return nil
}

func (s *KaspaApi) startBlockTemplateListener(ctx context.Context, blockReadyCb func()) {
	blockReadyChan := make(chan bool)
	err := s.kaspad.RegisterForNewBlockTemplateNotifications(func(_ *appmessage.NewBlockTemplateNotificationMessage) {
		blockReadyChan <- true
	})
	if err != nil {
		s.logger.Error("fatal: failed to register for block notifications from kaspa")
	}

	const tickerTime = 500 * time.Millisecond
	ticker := time.NewTicker(tickerTime)
	for {
		select {
		case <-ctx.Done():
			s.logger.Warn("context cancelled, stopping block update listener")
			return
		case <-blockReadyChan:
			blockReadyCb()
			ticker.Reset(tickerTime)
		case <-ticker.C: // timeout, manually check for new blocks
			blockReadyCb()
		}
	}
}
