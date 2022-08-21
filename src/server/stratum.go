package gostratum

type StratumEvent struct {
	Id      int    `json:"id"`
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type StratumResult struct {
	Id      int    `json:"id"`
	Version string `json:"jsonrpc"`
	Result  any    `json:"result"`
	Error   []any  `json:"error"`
}
