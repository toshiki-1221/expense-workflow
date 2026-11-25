// hashpw は seed 用のパスワードハッシュ（bcrypt）を出力する。
//
//	go run ./cmd/hashpw <password>
package main

import (
	"fmt"
	"os"

	"github.com/toshiki-1221/expense-workflow/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/hashpw <password>")
		os.Exit(1)
	}
	h, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(h)
}
