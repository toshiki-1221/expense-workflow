// Package config は環境変数を起動時に1回だけ読み、アプリの設定としてまとめる。
// 環境変数を読むのはこのパッケージだけにする。
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

// minJWTSecretLength は HS256 の鍵として必要な長さ（256ビット = 32バイト）。
const minJWTSecretLength = 32

// Config はアプリ全体の設定を表す。
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
	// TrustedProxies は X-Forwarded-For を信頼するプロキシ（IP または CIDR）。
	// 空なら誰も信頼せず、接続元のIPをそのまま使う。本番は ALB のサブネットを指定する。
	TrustedProxies []string
}

// Load は環境変数を読み、足りない設定があればエラーを返す。
func Load() (Config, error) {
	databaseURL, err := loadDatabaseURL()
	if err != nil {
		return Config{}, err
	}

	jwtSecret := os.Getenv("JWT_SECRET") // 鍵は前後の空白も値として扱う
	if len(jwtSecret) < minJWTSecretLength {
		return Config{}, fmt.Errorf("config: JWT_SECRET must be at least %d bytes (openssl rand -base64 48 で作る)", minJWTSecretLength)
	}

	jwtTTL := 24 * time.Hour
	if v := strings.TrimSpace(os.Getenv("JWT_TTL")); v != "" { // 例: "1h", "24h"
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("config: JWT_TTL: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("config: JWT_TTL must be positive")
		}
		jwtTTL = d
	}

	return Config{
		Port:        envOrDefault("PORT", "8080"),
		DatabaseURL: databaseURL,
		JWTSecret:   jwtSecret,
		JWTTTL:      jwtTTL,
		// 例: TRUSTED_PROXIES=10.0.0.0/16,10.1.0.0/16
		TrustedProxies: splitList(os.Getenv("TRUSTED_PROXIES")),
	}, nil
}

// splitList はカンマ区切りの値を分割し、空の要素を除く。空なら nil を返す。
func splitList(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// loadDatabaseURL は DATABASE_URL があればそれを使い、なければ DB_* から組み立てる。
func loadDatabaseURL() (string, error) {
	if databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL")); databaseURL != "" {
		return databaseURL, nil
	}

	host := strings.TrimSpace(os.Getenv("DB_HOST"))
	user := strings.TrimSpace(os.Getenv("DB_USER"))
	password := os.Getenv("DB_PASSWORD") // パスワードは前後の空白も値として扱う
	database := strings.TrimSpace(os.Getenv("DB_NAME"))
	if host == "" || user == "" || password == "" || database == "" {
		return "", errors.New("config: DATABASE_URL or DB_HOST, DB_USER, DB_PASSWORD, and DB_NAME must be set")
	}

	// url.URL で組み立てると、パスワードの記号（@ / # など）が自動でエスケープされる
	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, envOrDefault("DB_PORT", "5432")),
		Path:   "/" + database,
	}
	query := connectionURL.Query()
	query.Set("sslmode", envOrDefault("DB_SSLMODE", "disable")) // 本番では require 以上にする
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String(), nil
}

// envOrDefault は環境変数が空の場合に既定値を返す。
func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
