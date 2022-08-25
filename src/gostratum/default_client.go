package gostratum

import (
	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/pkg/errors"
)

func DefaultHandlers() StratumHandlerMap {
	return StratumHandlerMap{
		string(stratumrpc.StratumMethodSubscribe): HandleSubscribe,
		string(stratumrpc.StratumMethodAuthorize): HandleAuthorize,
		string(stratumrpc.StratumMethodSubmit):    HandleSubmit,
	}
}

func HandleAuthorize(ctx StratumContext, event stratumrpc.JsonRpcEvent) error {
	if err := ctx.Reply(stratumrpc.NewResponse(event, true, nil)); err != nil {
		return errors.Wrap(err, "failed to send response to authorize")
	}
	ctx.Logger.Info("client authorized")
	return nil
}

func HandleSubscribe(ctx StratumContext, event stratumrpc.JsonRpcEvent) error {
	if err := ctx.Reply(stratumrpc.NewResponse(event,
		[]any{true, "EthereumStratum/1.0.0"}, nil)); err != nil {
		return errors.Wrap(err, "failed to send response to subscribe")
	}
	ctx.Logger.Info("client subscribed")
	return nil
}

func HandleSubmit(ctx StratumContext, event stratumrpc.JsonRpcEvent) error {
	// stub
	ctx.Logger.Info("work submission")
	return nil
}
