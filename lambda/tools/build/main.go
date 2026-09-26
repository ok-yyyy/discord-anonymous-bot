// Command build はLambda用のGoバイナリをビルドする。
//
//	go run ./tools/build
//
// provided.al2023は実行ファイル名がbootstrapであることを要求するため、build/<関数名>/bootstrapという配置で出力する。
// CDKはこのディレクトリをそのままCode.fromAssetで参照する。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// targets はcmd配下のうちLambdaにデプロイするものを列挙する。
// registercmdはローカルから実行するCLIなので含めない。
var targets = []string{"interaction"}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "build failed:", err)
		os.Exit(1)
	}
}

func run() error {
	// 前回の成果物が残っていると、ビルドに失敗しても古いバイナリが
	// デプロイされうるので、毎回消してから作り直す。
	if err := os.RemoveAll("build"); err != nil {
		return fmt.Errorf("clean build dir: %w", err)
	}

	for _, t := range targets {
		out := filepath.Join("build", t, "bootstrap")
		if err := build(t, out); err != nil {
			return err
		}
		fmt.Println("built", out)
	}
	return nil
}

func build(target, out string) error {
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", out, "./cmd/"+target)
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=arm64",
		// 静的リンクにする。provided.al2023はバイナリを実行するだけなので、libcに依存させない。
		"CGO_ENABLED=0",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s: %w", target, err)
	}
	return nil
}
