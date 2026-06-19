package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/service/flight-search/internal/aggregator"
	"github.com/service/flight-search/internal/cache"
	"github.com/service/flight-search/internal/config"
	"github.com/service/flight-search/internal/handler"
	"github.com/service/flight-search/internal/middleware"
	"github.com/service/flight-search/internal/provider"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting flight search server",
		"port", cfg.Port,
		"cache_ttl", cfg.CacheTTL.String(),
		"provider_timeout", cfg.ProviderTimeout.String(),
		"rate_limit", cfg.RateLimit,
		"rate_burst", cfg.RateBurst,
		"max_retries", cfg.MaxRetries,
	)

	providers := []provider.FlightProvider{
		provider.NewGarudaProvider(),
		provider.NewLionAirProvider(),
		provider.NewBatikAirProvider(),
		provider.NewAirAsiaProvider(cfg.MaxRetries),
	}

	flightCache := cache.New(cfg.CacheTTL)
	agg := aggregator.New(providers, flightCache, cfg.ProviderTimeout)
	searchHandler := handler.NewSearchHandler(agg)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogMiddleware())

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit, cfg.RateBurst)
	r.Use(rateLimiter.Middleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		api.POST("/flights/search", searchHandler.HandleSearch)
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Give in-flight requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Cleanup background goroutines
	flightCache.Close()
	rateLimiter.Close()

	slog.Info("server exited")
}

func ginLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"client_ip", c.ClientIP(),
		)
	}
}
