package gostratum

import "encoding/json"

type JsonRpcEvent struct {
	Id      any           `json:"id"` // id can be nil, a string, or an int 🙄
	Version string        `json:"jsonrpc"`
	Method  StratumMethod `json:"method"`
	Params  []any         `json:"params"`
}

type JsonRpcResponse struct {
	Id     any   `json:"id"`
	Result any   `json:"result"`
	Error  []any `json:"error"`
}

func NewEvent(id string, method string, params []any) JsonRpcEvent {
	var finalId any
	if len(id) == 0 {
		finalId = nil
	} else {
		finalId = id
	}

	return JsonRpcEvent{
		Id:      finalId,
		Version: "2.0",
		Method:  StratumMethod(method),
		Params:  params,
	}
}

func NewResponse(event JsonRpcEvent, results any, err []any) JsonRpcResponse {
	return JsonRpcResponse{
		Id:     event.Id,
		Result: results,
		Error:  err,
	}
}

func UnmarshalEvent(in string) (JsonRpcEvent, error) {
	event := JsonRpcEvent{}
	if err := json.Unmarshal([]byte(in), &event); err != nil {
		return JsonRpcEvent{}, err
	}
	return event, nil
}

func UnmarshalResponse(in string) (JsonRpcResponse, error) {
	resp := JsonRpcResponse{}
	if err := json.Unmarshal([]byte(in), &resp); err != nil {
		return JsonRpcResponse{}, err
	}
	return resp, nil
}
