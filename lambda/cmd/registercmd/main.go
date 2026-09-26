// Command registercmd はDiscordにスラッシュコマンドを登録する。
//
//	go run ./cmd/registercmd
//
// ローカルから手で実行する。
// デプロイとは独立した操作で、コマンド定義を変えたときだけ実行すればよい。
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disgoorg/disgo/rest"

	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/config"
	"github.com/ok-yyyy/discord-anonymous-bot/lambda/internal/handler"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	// .envはリポジトリ直下にある。このCLIはlambda/から実行する。
	cfg, err := config.LoadRegister(filepath.Join("..", ".env"))
	if err != nil {
		return err
	}

	commands := handler.Definitions()
	client := rest.New(rest.NewClient(cfg.BotToken))

	// bulk overwriteで一括登録する。
	// ここに含めなかったコマンドは削除される。
	if _, err := client.SetGlobalCommands(cfg.ApplicationID, commands); err != nil {
		return fmt.Errorf("register commands: %w", err)
	}

	for _, c := range commands {
		fmt.Printf("registered /%s\n", c.CommandName())
	}
	return nil
}
