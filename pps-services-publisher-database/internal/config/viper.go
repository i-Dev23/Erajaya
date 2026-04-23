package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// NewViper loads configuration from config.json with env var override support (prefix: PPS_).
func NewViper() *viper.Viper {
	config := viper.New()

	config.SetConfigName("config")
	config.SetConfigType("json")
	config.AddConfigPath("./../")
	config.AddConfigPath("./")

	config.AutomaticEnv()
	config.SetEnvPrefix("PPS")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	return config
}
