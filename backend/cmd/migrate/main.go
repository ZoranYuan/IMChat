package main

import (
	"IM_backend/configs"
	"IM_backend/internal/infrastructure/persistence/mysql"
	"log"
	"os"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/workspace/IM/backend/configs/config.yaml"
	}

	cfg := configs.LoadConfig(configPath)
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置校验失败：%v", err)
	}
	db := mysql.InitMysql(cfg.Database.MySQL.DSN)
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取 MySQL 连接失败：%v", err)
	}
	if err := sqlDB.Close(); err != nil {
		log.Fatalf("关闭 MySQL 连接失败：%v", err)
	}
	log.Println("数据库迁移完成")
}
