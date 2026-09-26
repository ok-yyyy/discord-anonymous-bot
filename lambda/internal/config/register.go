package config

import (
	"fmt"
	"os"

	"github.com/disgoorg/snowflake/v2"
	"github.com/joho/godotenv"
)

// Register はコマンド登録CLIが必要とする設定。
type Register struct {
	BotToken      string       `env:"DISCORD_BOT_TOKEN,notEmpty"`
	ApplicationID snowflake.ID `env:"DISCORD_APPLICATION_ID,notEmpty"`
}

// LoadRegister はコマンド登録CLIの設定を読む。
//
// このCLIはローカルから実行し、Lambdaと違って環境変数が注入されないためenvFileの内容もプロセスの環境変数に読み込む。
//
// godotenvは既に設定されている環境変数を上書きしないので、シェルで一時的に差し替えられる。
// ファイルが無い場合も、環境変数が直接設定されていれば動く。
func LoadRegister(envFile string) (*Register, error) {
	if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load %s: %w", envFile, err)
	}
	return load[Register]()
}
