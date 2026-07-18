package kafka

import (
	"IM_backend/configs"

	"github.com/IBM/sarama"
)

type Client struct {
	Producer sarama.SyncProducer
	Consumer sarama.ConsumerGroup
}

func NewClient(cfg configs.KafkaConfig) (*Client, error) {
	pcfg := sarama.NewConfig()
	pcfg.Version = sarama.V2_6_0_0
	pcfg.Producer.Return.Successes = true
	pcfg.Producer.Retry.Max = cfg.Producer.Retries
	pcfg.Producer.RequiredAcks = sarama.WaitForAll
	pcfg.Producer.Idempotent = true
	pcfg.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducer(cfg.Brokers, pcfg)
	if err != nil {
		return nil, err
	}

	ccfg := sarama.NewConfig()
	ccfg.Version = sarama.V2_6_0_0
	ccfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategySticky(),
	}

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.Consumer.GroupID, ccfg)
	if err != nil {
		_ = producer.Close()
		return nil, err
	}

	return &Client{
		Producer: producer,
		Consumer: consumer,
	}, nil
}

func (client *Client) Close() error {
	if client == nil {
		return nil
	}

	var firstErr error
	if client.Consumer != nil {
		firstErr = client.Consumer.Close()
	}
	if client.Producer != nil {
		if err := client.Producer.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
