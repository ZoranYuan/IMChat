package kafka

import (
	"IM_backend/configs"

	"github.com/IBM/sarama"
)

type Client struct {
	Producer sarama.SyncProducer
	Consumer sarama.ConsumerGroup
	Cfg      configs.KafkaConfig
}

func NewClient(cfg configs.KafkaConfig) (*Client, error) {
	pcfg := sarama.NewConfig()
	pcfg.Producer.Return.Successes = true
	pcfg.Producer.Retry.Max = cfg.Producer.Retries
	pcfg.Producer.RequiredAcks = sarama.WaitForAll

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
		return nil, err
	}

	return &Client{
		Producer: producer,
		Consumer: consumer,
		Cfg:      cfg,
	}, nil
}
