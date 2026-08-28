package anonymous

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

// DiscordのアバターはSVG非対応のためPNGを指定する
const avatarURLFormat = "https://api.dicebear.com/10.x/shapes/png?size=128&seed=%s"

var namePrefix = []string{
	"あかい", "あおい", "きいろい", "みどりの", "しろい", "くろい",
	"ちいさな", "おおきな", "はやい", "ねむい", "げんきな", "しずかな",
	"ふしぎな", "やさしい", "つよい", "まるい", "ひかる", "とおい",
	"あまい", "すずしい", "あたたかい", "かるい", "おもい", "かしこい",
}

var nameSuffix = []string{
	"ねこ", "いぬ", "きつね", "たぬき", "うさぎ", "くま",
	"ぱんだ", "ぺんぎん", "いるか", "くじら", "とら", "らいおん",
	"さる", "りす", "ふくろう", "からす", "かえる", "かめ",
	"へび", "わし", "ひつじ", "うま", "ぞう", "きりん",
}

// anonymousIdentity はユーザIDと日付から匿名表示名とアバターURLを一意に決定する。
// salt を鍵としたHMACを使うことで、saltを知らない第三者がユーザIDから
// 匿名表示名を総当たりで逆引きできないようにしている。
func anonymousIdentity(salt, userID string, t time.Time) (username string, avatarURL string) {
	mac := hmac.New(sha256.New, []byte(salt))
	fmt.Fprintf(mac, "%s:%s", userID, t.Format("2006-01-02"))
	hash := mac.Sum(nil)

	a := namePrefix[binary.BigEndian.Uint64(hash[0:8])%uint64(len(namePrefix))]
	b := nameSuffix[binary.BigEndian.Uint64(hash[8:16])%uint64(len(nameSuffix))]
	seed := hex.EncodeToString(hash[16:32])

	return a + b, fmt.Sprintf(avatarURLFormat, seed)
}
