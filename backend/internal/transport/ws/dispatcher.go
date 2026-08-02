package ws

import (
	realtimews "IM_backend/internal/infrastructure/realtime/websocket"
	"context"
	"log"
)

type DispatchHandle func(ctx context.Context, session *realtimews.Session, data []byte) error

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

func (r *Dispatcher) Dispatch(ctx context.Context, session *realtimews.Session, op string, data []byte) {
	handler, ok := r.handlers[op]
	if !ok {
		log.Printf("未知的消息类型 %s", op)
		return
	}

	ctx = context.WithValue(ctx, "op", op)
	err := handler(ctx, session, data)

	if err != nil {
		log.Println("处理 WebSocket 消息失败：", err)
		// TODO 对错误进行补偿措施
	}
}
