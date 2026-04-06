package ws

import (
	"context"
	"encoding/json"
	"log"
)

type DispatchHandle func(ctx context.Context, client *Client, data json.RawMessage) error

type Dispatcher struct {
	handlers map[string]DispatchHandle
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]DispatchHandle),
	}
}

func (d *Dispatcher) RegisterHandler(op string, dh DispatchHandle) {
	d.handlers[op] = dh
}

func (r *Dispatcher) Dispatch(ctx context.Context, client *Client, msgType string, data json.RawMessage) {
	handler, ok := r.handlers[msgType]
	if !ok {
		// 未知类型，可以打印日志或忽略
		return
	}

	err := handler(ctx, client, data)

	if err != nil {
		log.Println("failed to handle message, ", err)
		// TODO 对错误进行补偿措施
	}
}
