package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/Monthlyaway/short-link/internal/cache"
	"github.com/Monthlyaway/short-link/internal/filter"
	"github.com/Monthlyaway/short-link/internal/model"
	"github.com/Monthlyaway/short-link/internal/repository"
	"github.com/Monthlyaway/short-link/internal/utils"
)

// URLService handles business logic for URL shortening
type URLService struct {
	repo   *repository.URLRepository
	cache  *cache.RedisCache
	bloom  *filter.BloomFilter
}

// NewURLService creates a new URL service instance
func NewURLService(repo *repository.URLRepository, cache *cache.RedisCache, bloom *filter.BloomFilter) *URLService {
	return &URLService{
		repo:  repo,
		cache: cache,
		bloom: bloom,
	}
}

// CreateShortURL creates a new short URL
func (s *URLService) CreateShortURL(ctx context.Context, originalURL string, expiredAt *time.Time) (*model.URLMapping, error) {
	// Validate URL
	if err := s.validateURL(originalURL); err != nil {
		return nil, err
	}

	// Check if the URL already exists
	existing, err := s.repo.GetByOriginalURL(ctx, originalURL)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.IsActive() {
		return existing, nil
	}

	// Generate short code
	shortCode, err := utils.GenerateShortCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate short code: %w", err)
	}

	// Check for collision (very unlikely with snowflake)
	for i := 0; i < 3; i++ {
		exists, err := s.repo.GetByShortCode(ctx, shortCode)
		if err != nil {
			return nil, err
		}
		if exists == nil {
			break
		}
		// Generate a new short code if collision detected
		shortCode, err = utils.GenerateShortCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate short code: %w", err)
		}
	}

	// Create URL mapping
	mapping := &model.URLMapping{
		ShortCode:   shortCode,
		OriginalURL: originalURL,
		ExpiredAt:   expiredAt,
		Status:      1,
	}

	if err := s.repo.Create(ctx, mapping); err != nil {
		return nil, err
	}

	// Update cache and bloom filter
	if err := s.cache.Set(ctx, shortCode, originalURL); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to set cache: %v\n", err)
	}
	s.bloom.Add(shortCode)

	return mapping, nil
}

// GetOriginalURL retrieves the original URL by short code
// Uses cascade: Bloom filter -> Redis -> MySQL
func (s *URLService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	// Check bloom filter first
	if !s.bloom.Test(shortCode) {
		return "", fmt.Errorf("short code not found")
	}

	// Check Redis cache
	originalURL, err := s.cache.Get(ctx, shortCode)
	if err != nil {
		fmt.Printf("Failed to get from cache: %v\n", err)
	}
	if originalURL != "" {
		return originalURL, nil
	}

	// Check database
	mapping, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return "", err
	}
	if mapping == nil {
		return "", fmt.Errorf("short code not found")
	}

	// Check if active
	if !mapping.IsActive() {
		return "", fmt.Errorf("short code is expired or disabled")
	}

	// Update cache
	if err := s.cache.Set(ctx, shortCode, mapping.OriginalURL); err != nil {
		fmt.Printf("Failed to set cache: %v\n", err)
	}

	return mapping.OriginalURL, nil
}

// GetURLInfo retrieves URL mapping information by short code
func (s *URLService) GetURLInfo(ctx context.Context, shortCode string) (*model.URLMapping, error) {
	mapping, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}
	if mapping == nil {
		return nil, fmt.Errorf("short code not found")
	}
	return mapping, nil
}

// RecordVisit records a visit to a short URL
func (s *URLService) RecordVisit(ctx context.Context, shortCode, ip, userAgent string) error {
	// Increment visit count asynchronously
	go func() {
		if err := s.repo.IncrementVisitCount(context.Background(), shortCode); err != nil {
			fmt.Printf("Failed to increment visit count: %v\n", err)
		}
	}()

	// Create visit log asynchronously
	go func() {
		log := &model.VisitLog{
			ShortCode: shortCode,
			IP:        ip,
			UserAgent: userAgent,
		}
		if err := s.repo.CreateVisitLog(context.Background(), log); err != nil {
			fmt.Printf("Failed to create visit log: %v\n", err)
		}
	}()

	return nil
}

// InitBloomFilter initializes the bloom filter with all existing short codes
func (s *URLService) InitBloomFilter(ctx context.Context) error {
	shortCodes, err := s.repo.GetAllShortCodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all short codes: %w", err)
	}

	s.bloom.AddBatch(shortCodes)
	fmt.Printf("Initialized bloom filter with %d short codes\n", len(shortCodes))

	return nil
}

// validateURL validates the URL format
func (s *URLService) validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	return nil
}
