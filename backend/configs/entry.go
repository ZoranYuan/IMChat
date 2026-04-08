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
		log.Fatal("failed to load config file ", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatal("failed to load config file ", err)
	}

	return config
}
