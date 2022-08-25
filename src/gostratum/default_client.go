package gostratum

import "go.uber.org/zap"

type StratumState int

const (
	StratumStateCreated StratumState = iota
	StratumStateAuthorized
)

type DefaultClient struct {
	proto *StratumClientProtocol
	state StratumState
}

func NewDefaultClient(proto *StratumClientProtocol) *DefaultClient {
	return &DefaultClient{
		proto: proto,
		state: StratumStateCreated,
	}
}

func (d *DefaultClient) OnAuthorize(params []any) {
	d.proto.logger.Info("authorized", zap.Any("params", params))
	d.state = StratumStateAuthorized
}
