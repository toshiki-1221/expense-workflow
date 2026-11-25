// Package auth はパスワードのハッシュ化とJWTの発行・検証を提供する。
package auth

import (
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost は OWASP の最低ライン 10 より上げた値。1つ上げると約2倍遅くなる。
// サーバーで1回 1 秒未満に収まる範囲で決める。
const bcryptCost = 12

// MaxPasswordBytes は bcrypt が扱える上限。文字数ではなくバイト数（日本語は1文字3バイト）。
const MaxPasswordBytes = 72

// HashPassword はパスワードをbcryptでハッシュ化する。seed やユーザー登録で使う。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err // 72バイトを超えると bcrypt.ErrPasswordTooLong
	}
	return string(b), nil
}

// CheckPassword はハッシュと平文が一致すれば true を返す。
func CheckPassword(hash, plain string) bool {
	// CompareHashAndPassword は 72 バイトを超えた部分を黙って無視するので、先に弾く。
	if len(plain) > MaxPasswordBytes {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// dummyHash はユーザーが存在しないときも同じだけ時間をかけるためのダミー。
// sync.OnceValue で初回だけ作る（パッケージ初期化で重い処理・エラー握りつぶしをしない）。
var dummyHash = sync.OnceValue(func() string {
	h, err := HashPassword("dummy-password-for-timing")
	if err != nil {
		panic(err) // 固定文字列なので起きない
	}
	return h
})

// CheckDummy は「ユーザーが見つからなかった」ときに呼ぶ。
// これが無いと、応答の速さで「そのメールアドレスが登録済みか」が分かってしまう。
func CheckDummy(plain string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash()), []byte(plain))
}
