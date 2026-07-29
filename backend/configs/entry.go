package configs

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	App       App             `yaml:"app"`
	Server    Server          `yaml:"server"`
	Database  Database        `yaml:"database"`
	JWT       JWT             `yaml:"jwt"`
	WebSocket WebSocketConfig `yaml:"ws"`
	Message   MessageConfig   `yaml:"message"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	Storage   StorageConfig   `yaml:"storage"`
	Cache     CacheConfig     `yaml:"cache"`
}

type UserProfile struct {
	NegativeTTL int `yaml:"negative_ttl"`
	TTL         int `yaml:"ttl"`
}

type CacheConfig struct {
	UserProfile UserProfile `yaml:"user_profile"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`

	Topics KafkaTopics `yaml:"topics"`

	Consumer KafkaConsumerConfig `yaml:"consumer"`

	Producer KafkaProducerConfig `yaml:"producer"`

	Partition KafkaPartitionConfig `yaml:"partition"`
}

type KafkaTopics struct {
	Chat string `yaml:"chat"`
	Ack  string `yaml:"ack"`
}

type KafkaConsumerConfig struct {
	GroupID  string `yaml:"group_id"`
	Version  string `yaml:"version"`
	Assignor string `yaml:"assignor"` // range / roundrobin / sticky
}

type KafkaProducerConfig struct {
	Acks        string `yaml:"acks"`
	Retries     int    `yaml:"retries"`
	BatchSize   int    `yaml:"batch_size"`
	LingerMs    int    `yaml:"linger_ms"`
	Compression string `yaml:"compression"`
}

type KafkaPartitionConfig struct {
	Strategy string `yaml:"strategy"` // conversation_hash / user_hash / random
}

type StorageConfig struct {
	MinIO MinIOConfig `yaml:"minio"`
}

type MinIOConfig struct {
	Endpoint                        string `yaml:"endpoint"`
	PublicEndpoint                  string `yaml:"public_endpoint"`
	AccessKeyID                     string `yaml:"access_key_id"`
	SecretAccessKey                 string `yaml:"secret_access_key"`
	Bucket                          string `yaml:"bucket"`
	UseSSL                          bool   `yaml:"use_ssl"`
	CacheTTLSeconds                 int    `yaml:"cache_ttl_seconds"`
	URLTTLSeconds                   int    `yaml:"url_ttl_seconds"`
	MultipartTTL                    int    `yaml:"multipartTTL"`
	PartURLTTLSeconds               int    `yaml:"part_url_ttl_seconds"`
	MultipartInitLockTTLSeconds     int    `yaml:"multipart_init_lock_ttl_seconds"`
	MultipartCompleteLockTTLSeconds int    `yaml:"multipart_complete_lock_ttl_seconds"`
}

type App struct {
	Name      string `yaml:"name"`
	Version   string `yaml:"version"`
	Env       string `yaml:"env"`
	MachineID int64  `yaml:"machineId"`
}

type WebSocketConfig struct {
	WriteWaitSeconds         int `yaml:"write_wait_seconds"`
	PongWaitSeconds          int `yaml:"pong_wait_seconds"`
	PingPeriodSeconds        int `yaml:"ping_period_seconds"`
	TimerInterval            int `yaml:"timer_interval_seconds"`
	MaxMessageSize           int `yaml:"max_message_size"`
	MaxMessageSendBufferSize int `yaml:"max_message_send_buffer_size"`
	SendMessageRate          int `yaml:"send_message_rate"`
	SendMessageBurst         int `yaml:"send_message_burst"`
	BatchMaxMessages         int `yaml:"batch_max_messages"`
	BatchMaxBytes            int `yaml:"batch_max_bytes"`
	BatchLingerMilliseconds  int `yaml:"batch_linger_milliseconds"`
	BatchReadyQueueSize      int `yaml:"batch_ready_queue_size"`
}

type MessageConfig struct {
	RoomRealtimeFanoutLimit           int `yaml:"room_realtime_fanout_limit"`
	LargeRoomNoticeLingerMilliseconds int `yaml:"large_room_notice_linger_milliseconds"`
	LargeRoomNoticeShardCount         int `yaml:"large_room_notice_shard_count"`
	LargeRoomNoticeMaxPending         int `yaml:"large_room_notice_max_pending"`
	HistoryDefaultLimit               int `yaml:"history_default_limit"`
	HistoryMaxLimit                   int `yaml:"history_max_limit"`
	SyncDefaultLimit                  int `yaml:"sync_default_limit"`
	SyncMaxLimit                      int `yaml:"sync_max_limit"`
}

type Server struct {
	Port string `yaml:"port"`
}

type Database struct {
	MySQL MySQL `yaml:"mysql"`
	Redis Redis `yaml:"redis"`
}

type Redis struct {
	DSN string `yaml:"dsn"`
}

type MySQL struct {
	DSN string `yaml:"dsn"`
}

type JWT struct {
	Secret              string `yaml:"secret"`
	AccessExpireMinutes int    `yaml:"access_expire_minutes"`
	RefreshExpireHours  int    `yaml:"refresh_expire_hours"`
}

func LoadConfig(path string) Config {
	data, err := os.ReadFile(path)

	if err != nil {
		log.Fatal("加载配置文件失败：", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatal("解析配置文件失败：", err)
	}

	return config
}
