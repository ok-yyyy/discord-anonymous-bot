package anon

import (
	"strings"
	"testing"
	"time"
)

const salt = "test-salt"

var noon = time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

func TestDeriveIsDeterministic(t *testing.T) {
	if Derive("111", noon, salt) != Derive("111", noon.Add(time.Hour), salt) {
		t.Error("same user on the same day got different identities")
	}
}

func TestDeriveDiffersPerUser(t *testing.T) {
	// 表示名は衝突しうるので、128ビット由来のアイコンで判定する。
	if Derive("111", noon, salt).AvatarURL == Derive("222", noon, salt).AvatarURL {
		t.Error("different users got the same avatar")
	}
}

func TestDeriveChangesWithSalt(t *testing.T) {
	if Derive("111", noon, salt) == Derive("111", noon, "other-salt") {
		t.Error("identity did not change when the salt changed")
	}
}

func TestDeriveChangesNextDay(t *testing.T) {
	if Derive("111", noon, salt) == Derive("111", noon.Add(24*time.Hour), salt) {
		t.Error("identity did not change on the next day")
	}
}

// 日付はUTCで丸める。nowのロケーションに引きずられると境界がずれる。
func TestDeriveRollsOverAtUTCMidnight(t *testing.T) {
	// どちらも 2026-09-18 09:00 UTC。表記しているゾーンだけが違う。
	utc := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	jst := utc.In(time.FixedZone("JST", 9*60*60)) // 現地では 09-18 18:00
	if Derive("111", utc, salt) != Derive("111", jst, salt) {
		t.Error("the same instant in a different location produced a different identity")
	}

	// 23:30 UTC と 00:30 UTC は別の日。
	before := time.Date(2026, 9, 18, 23, 30, 0, 0, time.UTC)
	after := time.Date(2026, 9, 19, 0, 30, 0, 0, time.UTC)
	if Derive("111", before, salt) == Derive("111", after, salt) {
		t.Error("identity did not change across UTC midnight")
	}
}

func TestDeriveUsesWordsFromTheLists(t *testing.T) {
	name := Derive("111", noon, salt).Name

	if !hasAnyPrefix(name, namePrefix) {
		t.Errorf("name %q does not start with a word from namePrefix", name)
	}
	if !hasAnySuffix(name, nameSuffix) {
		t.Errorf("name %q does not end with a word from nameSuffix", name)
	}
}

// seedは16バイトをhexにしたものなので32文字になる。
func TestDeriveAvatarURL(t *testing.T) {
	const prefix = "https://api.dicebear.com/10.x/shapes/png?size=128&seed="

	seed, ok := strings.CutPrefix(Derive("111", noon, salt).AvatarURL, prefix)
	if !ok {
		t.Fatalf("avatar URL does not start with %q", prefix)
	}
	if len(seed) != 32 {
		t.Errorf("seed length = %d, want 32", len(seed))
	}
}

// 導出結果にuserIDやsaltが残っていないことを確かめる。
func TestDeriveDoesNotLeakInput(t *testing.T) {
	const userID = "123456789012345678"

	id := Derive(userID, noon, salt)
	for _, secret := range []string{userID, salt} {
		if strings.Contains(id.Name, secret) || strings.Contains(id.AvatarURL, secret) {
			t.Errorf("identity %+v contains %q", id, secret)
		}
	}
}

func TestIndexStaysInRange(t *testing.T) {
	max := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	for _, n := range []int{1, 3, 24, 64} {
		if got := index(max, n); got < 0 || got >= n {
			t.Errorf("index(max, %d) = %d, want 0 <= i < %d", n, got, n)
		}
	}
}

// 語彙リストが空だとDeriveがパニックし、重複があるとその語だけ出やすくなる。
func TestWordLists(t *testing.T) {
	for _, list := range []struct {
		name  string
		words []string
	}{
		{"namePrefix", namePrefix},
		{"nameSuffix", nameSuffix},
	} {
		if len(list.words) == 0 {
			t.Errorf("%s is empty", list.name)
		}

		seen := make(map[string]bool, len(list.words))
		for _, w := range list.words {
			if seen[w] {
				t.Errorf("%s contains a duplicate: %q", list.name, w)
			}
			seen[w] = true
		}
	}
}

func hasAnyPrefix(s string, candidates []string) bool {
	for _, c := range candidates {
		if strings.HasPrefix(s, c) {
			return true
		}
	}
	return false
}

func hasAnySuffix(s string, candidates []string) bool {
	for _, c := range candidates {
		if strings.HasSuffix(s, c) {
			return true
		}
	}
	return false
}
