package config

import "github.com/disgoorg/snowflake/v2"

// Worker はworker Lambdaが必要とする設定。
type Worker struct {
	BotToken      string       `env:"DISCORD_BOT_TOKEN,notEmpty"`
	ApplicationID snowflake.ID `env:"DISCORD_APPLICATION_ID,notEmpty"`
	// AnonymousSalt は匿名IDの導出に使う。ログに出さないこと。
	AnonymousSalt string `env:"ANONYMOUS_SALT,notEmpty"`
}

// LoadWorker はworker Lambdaの設定を環境変数から読む。
func LoadWorker() (*Worker, error) {
	return load[Worker]()
}
