package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/toshiki-1221/expense-workflow/internal/db"
	"github.com/toshiki-1221/expense-workflow/internal/handler"
	"github.com/toshiki-1221/expense-workflow/internal/repository"
	"github.com/toshiki-1221/expense-workflow/internal/service"
)

// main はDB接続とHTTPサーバーを起動する。
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run は依存関係を組み立ててAPIサーバーを実行する。
func run() error {
	ctx := context.Background()
	databaseURL, err := buildDatabaseURL()
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	queries := db.New(pool)
	categoryRepository := repository.NewCategoryRepository(queries)
	categoryService := service.NewCategoryService(categoryRepository)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	r := gin.Default()

	v1 := r.Group("/api/v1")
	v1.GET("/categories", categoryHandler.List)

	if err := r.Run(":" + envOrDefault("PORT", "8080")); err != nil {
		return fmt.Errorf("run HTTP server: %w", err)
	}
	return nil
}

// buildDatabaseURL は環境変数からPostgreSQL接続URLを作成する。
func buildDatabaseURL() (string, error) {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL, nil
	}

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	database := os.Getenv("DB_NAME")
	if host == "" || user == "" || password == "" || database == "" {
		return "", fmt.Errorf("DATABASE_URL or DB_HOST, DB_USER, DB_PASSWORD, and DB_NAME must be set")
	}

	port := envOrDefault("DB_PORT", "5432")
	connectionURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + database,
	}
	query := connectionURL.Query()
	query.Set("sslmode", "disable")
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
