package mq

type Producer struct {
}

func NewProducer() *Producer {
	return &Producer{}
}

func (p *Producer) Send(toUserID string, payload any) {}
