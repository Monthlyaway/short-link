package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Monthlyaway/short-link/config"
	"github.com/Monthlyaway/short-link/internal/cache"
	"github.com/Monthlyaway/short-link/internal/filter"
	"github.com/Monthlyaway/short-link/internal/handler"
	"github.com/Monthlyaway/short-link/internal/repository"
	"github.com/Monthlyaway/short-link/internal/service"
	"github.com/Monthlyaway/short-link/internal/utils"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Snowflake ID generator
	if err := utils.InitSnowflake(cfg.Snowflake.DatacenterID, cfg.Snowflake.WorkerID); err != nil {
		log.Fatalf("Failed to initialize Snowflake: %v", err)
	}

	// Initialize MySQL repository
	repo, err := repository.NewURLRepository(
		cfg.MySQL.DSN(),
		cfg.MySQL.MaxIdleConns,
		cfg.MySQL.MaxOpenConns,
	)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize Redis cache
	redisCache, err := cache.NewRedisCache(
		cfg.Redis.Addr(),
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.PoolSize,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Initialize Bloom filter
	bloomFilter := filter.NewBloomFilter(
		cfg.BloomFilter.Capacity,
		cfg.BloomFilter.FalsePositiveRate,
	)

	// Initialize URL service
	urlService := service.NewURLService(repo, redisCache, bloomFilter)

	// Load all short codes into bloom filter
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := urlService.InitBloomFilter(ctx); err != nil {
		log.Printf("Warning: Failed to initialize bloom filter: %v", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize Gin router
	router := gin.Default()

	// Build base URL
	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)

	// Initialize handler
	urlHandler := handler.NewURLHandler(urlService, baseURL)

	// Register routes
	router.GET("/health", urlHandler.HealthCheck)
	router.GET("/:short_code", urlHandler.RedirectToOriginalURL)

	api := router.Group("/api/v1")
	{
		api.POST("/shorten", urlHandler.CreateShortURL)
		api.GET("/info/:short_code", urlHandler.GetURLInfo)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on port %d...", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
