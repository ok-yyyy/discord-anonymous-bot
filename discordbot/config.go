package discordbot

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	BotToken      string `env:"DISCORD_BOT_TOKEN,required"`
	AnonymousSalt string `env:"ANONYMOUS_SALT,required"`
}

func LoadConfig() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	return cfg, nil
}
