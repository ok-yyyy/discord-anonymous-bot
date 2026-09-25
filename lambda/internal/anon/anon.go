// Package anon は投稿者から匿名の表示名とアイコンを導出する。
//
// 投稿者の識別子は保存せず、投稿のたびにハッシュから決定的に求める。
// 日付を混ぜているため、日が変われば同じ利用者でも別の匿名IDになり、日をまたいだ追跡ができない。
package anon

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

// avatarURLFormat はアイコン画像の生成先。%sにはseedが入る。
const avatarURLFormat = "https://api.dicebear.com/10.x/shapes/png?size=128&seed=%s"

// Identity は匿名化された投稿者。Webhookのusername/avatar_urlにそのまま渡す。
type Identity struct {
	Name      string
	AvatarURL string
}

// Derive は利用者IDと時刻から匿名の表示名とアイコンを導出する。
//
// ハッシュ32バイトの使い道:
//
//	[0:8]   名前のプレフィックスのインデックス
//	[8:16]  名前のサフィックスのインデックス
//	[16:32] アイコン生成のseed
//
// 日付はUTCで丸める。
// nowのロケーションに依存させると、呼び出し側が渡す時刻の場所によって境界がずれるため。
func Derive(userID string, now time.Time, salt string) Identity {
	sum := sha256.Sum256([]byte(userID + ":" + now.UTC().Format("2006-01-02") + ":" + salt))

	a := namePrefix[index(sum[0:8], len(namePrefix))]
	b := nameSuffix[index(sum[8:16], len(nameSuffix))]
	seed := hex.EncodeToString(sum[16:32])

	return Identity{
		Name:      a + b,
		AvatarURL: fmt.Sprintf(avatarURLFormat, seed),
	}
}

// indexは8バイトを語彙リストの添字に変換する。
func index(b []byte, n int) int {
	return int(binary.BigEndian.Uint64(b) % uint64(n))
}
