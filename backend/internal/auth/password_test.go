package auth

import (
	"strings"
	"testing"
)

func TestCheckPassword(t *testing.T) {
	long72 := strings.Repeat("a", MaxPasswordBytes)
	hash, err := HashPassword(long72)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		plain string
		want  bool
	}{
		{"一致する", long72, true},
		{"違うパスワード", "wrong", false},
		// bcrypt は 72 バイトより後ろを見ないので、ここで弾けないと通ってしまう
		{"72バイトを超えると先頭が一致しても拒否", long72 + "extra", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckPassword(hash, tt.plain); got != tt.want {
				t.Fatalf("CheckPassword() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestHashPassword_TooLong(t *testing.T) {
	// 日本語は1文字3バイト。25文字 = 75バイトで上限を超える
	if _, err := HashPassword(strings.Repeat("あ", 25)); err == nil {
		t.Fatal("err = nil; want ErrPasswordTooLong")
	}
}
