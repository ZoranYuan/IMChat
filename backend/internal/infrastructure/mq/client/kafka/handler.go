package kafka

import (
	"IM_backend/internal/protocol"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
)

type groupHandler struct {
	dispacth Dispatch
}

func (h *groupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *groupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *groupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {
		// 将数据进行拆分 msg.Value
		var envelope protocol.Envelope

		if err := json.Unmarshal(msg.Value, &envelope); err != nil {
			session.MarkMessage(msg, "")
			return err
		}

		log.Println("read message")

		to := envelope.To
		if err := h.dispacth.SendToClient(msg.Topic, to, envelope.Payload); err != nil {
			// 让消息重试
			log.Println("failed to send message, ", err)
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
