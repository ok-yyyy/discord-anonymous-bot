package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEnvFile はテスト用の.envを作る。
func writeEnvFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// unsetEnv は環境変数を一時的に外し、テスト終了時に元へ戻す。
//
// godotenvは既に設定されている変数を上書きしない。
// ファイルから読む経路を試すには本当に未設定にする必要がある。
// t.Setenvで空文字を入れるだけでは「設定済み」と見なされて読み込まれない。
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if old, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, old) })
		} else {
			t.Cleanup(func() { os.Unsetenv(key) })
		}
		os.Unsetenv(key)
	}
}

// このCLIはローカル実行で環境変数が注入されないため、.envから読めること。
func TestLoadRegisterReadsEnvFile(t *testing.T) {
	unsetEnv(t, "DISCORD_BOT_TOKEN", "DISCORD_APPLICATION_ID")

	path := writeEnvFile(t, strings.Join([]string{
		"# コメントは無視する",
		"",
		`DISCORD_BOT_TOKEN="quoted-token"`,
		"DISCORD_APPLICATION_ID=123456789012345678",
		"ANONYMOUS_SALT=unused-here",
	}, "\n"))

	cfg, err := LoadRegister(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.BotToken != "quoted-token" {
		t.Errorf("bot token = %q, want quoted-token", cfg.BotToken)
	}
	if cfg.ApplicationID.String() != "123456789012345678" {
		t.Errorf("application id = %s, want 123456789012345678", cfg.ApplicationID)
	}
}

// シェルで一時的に差し替えられるよう、プロセスの環境変数を優先する。
func TestLoadRegisterPrefersProcessEnv(t *testing.T) {
	unsetEnv(t, "DISCORD_APPLICATION_ID")
	t.Setenv("DISCORD_BOT_TOKEN", "from-process")

	path := writeEnvFile(t, "DISCORD_BOT_TOKEN=from-file\nDISCORD_APPLICATION_ID=1\n")

	cfg, err := LoadRegister(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "from-process" {
		t.Errorf("bot token = %q, want from-process", cfg.BotToken)
	}
}

// 環境変数が直接設定されていれば.envは無くてもよい。
func TestLoadRegisterWithoutEnvFile(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "token")
	t.Setenv("DISCORD_APPLICATION_ID", "1")

	if _, err := LoadRegister(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRegisterRejectsMissingValues(t *testing.T) {
	unsetEnv(t, "DISCORD_BOT_TOKEN", "DISCORD_APPLICATION_ID")

	_, err := LoadRegister(writeEnvFile(t, ""))
	if err == nil {
		t.Fatal("want an error")
	}
	// 1つずつ直して再実行する手戻りを避けるため、どちらも報告されること。
	for _, key := range []string{"DISCORD_BOT_TOKEN", "DISCORD_APPLICATION_ID"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error = %q, want it to name %s", err, key)
		}
	}
}

// 空文字はnotEmptyで弾く。requiredだと通ってしまう。
func TestLoadRegisterRejectsEmptyValue(t *testing.T) {
	unsetEnv(t, "DISCORD_APPLICATION_ID")
	t.Setenv("DISCORD_BOT_TOKEN", "")

	if _, err := LoadRegister(writeEnvFile(t, "DISCORD_APPLICATION_ID=1\n")); err == nil {
		t.Error("want an error for an empty bot token")
	}
}

// 応用IDが数値でなければ起動時に弾く。
func TestLoadRegisterRejectsNonNumericApplicationID(t *testing.T) {
	unsetEnv(t, "DISCORD_BOT_TOKEN", "DISCORD_APPLICATION_ID")

	path := writeEnvFile(t, "DISCORD_BOT_TOKEN=token\nDISCORD_APPLICATION_ID=not-a-snowflake\n")

	if _, err := LoadRegister(path); err == nil {
		t.Error("want an error for a non-numeric application id")
	}
}
