package appserver

import (
	"context"
	"encoding/json"
)

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RemoteError    `json:"error,omitempty"`
}

type RemoteError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type Transport interface {
	Exchange(context.Context, []byte, int) ([]byte, error)
	Notify(context.Context, []byte) error
	Close() error
}
