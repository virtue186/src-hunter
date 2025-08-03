package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Logger   LoggerConfig
	Database DatabaseConfig
	Redis    RedisConfig
}

type ServerConfig struct {
	Port string
}

type LoggerConfig struct {
	Mode       string
	Level      string
	Path       string
	MaxSize    int `mapstructure:"max_size"`
	MaxBackups int `mapstructure:"max_backups"`
	MaxAge     int `mapstructure:"max_age"`
	Compress   bool
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// 全局配置变量
var Cfg *Config

// LoadConfig 从 configs/config.yaml 加载配置
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")       // 配置文件名 (不带后缀)
	viper.SetConfigType("yaml")         // 配置文件类型
	viper.AddConfigPath("./pkg/config") // 配置文件路径

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	Cfg = &cfg
	return &cfg, nil
}
