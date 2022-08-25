package testmocks

import (
	"encoding/json"

	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
)

func NewAuthorizeEvent() string {
	event := stratumrpc.NewEvent("1", "mining.authorize", []any{
		"", "test",
	})

	encoded, _ := json.Marshal(event)
	return string(encoded)
}
