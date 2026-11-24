package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/toshiki-1221/expense-workflow/internal/auth"
	"github.com/toshiki-1221/expense-workflow/internal/config"
	"github.com/toshiki-1221/expense-workflow/internal/db"
	"github.com/toshiki-1221/expense-workflow/internal/handler"
	"github.com/toshiki-1221/expense-workflow/internal/middleware"
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

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	queries := db.New(pool)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)

	userRepository := repository.NewUserRepository(queries)
	authService := service.NewAuthService(userRepository, tokens)
	authHandler := handler.NewAuthHandler(authService)

	categoryRepository := repository.NewCategoryRepository(queries)
	categoryService := service.NewCategoryService(categoryRepository)
	categoryHandler := handler.NewCategoryHandler(categoryService)

	// 申請と明細をまとめて保存するので、トランザクション用に pool も渡す
	reportRepository := repository.NewReportRepository(pool, queries)
	reportService := service.NewReportService(reportRepository)
	reportHandler := handler.NewReportHandler(reportService)

	r := gin.Default()
	// 既定ではすべてのプロキシを信頼し、X-Forwarded-For で c.ClientIP() を偽装できてしまう。
	// nil のときは誰も信頼せず、接続元のIPを使う。
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return fmt.Errorf("set trusted proxies: %w", err)
	}

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", authHandler.Login) // 認証不要はここだけ

	// ここから下は認証必須
	authed := v1.Group("", middleware.Authenticate(tokens))
	authed.GET("/me", authHandler.Me)
	authed.GET("/categories", categoryHandler.List)
	authed.GET("/reports", reportHandler.List)
	authed.POST("/reports", reportHandler.Create)
	// 経理APIを作るとき:
	// accounting := authed.Group("/accounting", middleware.RequireRole(authService, domain.RoleAccounting))

	if err := r.Run(":" + cfg.Port); err != nil {
		return fmt.Errorf("run HTTP server: %w", err)
	}
	return nil
}
