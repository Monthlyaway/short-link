package repository

import (
	"context"
	"fmt"

	"github.com/Monthlyaway/short-link/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// URLRepository handles database operations for URL mappings
type URLRepository struct {
	db *gorm.DB
}

// NewURLRepository creates a new URL repository instance
func NewURLRepository(dsn string, maxIdleConns, maxOpenConns int) (*URLRepository, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	// Auto-migrate tables
	if err := db.AutoMigrate(&model.URLMapping{}, &model.VisitLog{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &URLRepository{db: db}, nil
}

// Create creates a new URL mapping
func (r *URLRepository) Create(ctx context.Context, mapping *model.URLMapping) error {
	if err := r.db.WithContext(ctx).Create(mapping).Error; err != nil {
		return fmt.Errorf("failed to create URL mapping: %w", err)
	}
	return nil
}

// GetByShortCode retrieves a URL mapping by short code
func (r *URLRepository) GetByShortCode(ctx context.Context, shortCode string) (*model.URLMapping, error) {
	var mapping model.URLMapping
	if err := r.db.WithContext(ctx).Where("short_code = ?", shortCode).First(&mapping).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get URL mapping: %w", err)
	}
	return &mapping, nil
}

// GetByOriginalURL retrieves a URL mapping by original URL
func (r *URLRepository) GetByOriginalURL(ctx context.Context, originalURL string) (*model.URLMapping, error) {
	var mapping model.URLMapping
	if err := r.db.WithContext(ctx).Where("original_url = ?", originalURL).First(&mapping).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get URL mapping: %w", err)
	}
	return &mapping, nil
}

// IncrementVisitCount increments the visit count for a short code
func (r *URLRepository) IncrementVisitCount(ctx context.Context, shortCode string) error {
	if err := r.db.WithContext(ctx).Model(&model.URLMapping{}).
		Where("short_code = ?", shortCode).
		UpdateColumn("visit_count", gorm.Expr("visit_count + ?", 1)).Error; err != nil {
		return fmt.Errorf("failed to increment visit count: %w", err)
	}
	return nil
}

// CreateVisitLog creates a new visit log entry
func (r *URLRepository) CreateVisitLog(ctx context.Context, log *model.VisitLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to create visit log: %w", err)
	}
	return nil
}

// GetAllShortCodes retrieves all short codes from the database
func (r *URLRepository) GetAllShortCodes(ctx context.Context) ([]string, error) {
	var shortCodes []string
	if err := r.db.WithContext(ctx).Model(&model.URLMapping{}).
		Pluck("short_code", &shortCodes).Error; err != nil {
		return nil, fmt.Errorf("failed to get all short codes: %w", err)
	}
	return shortCodes, nil
}

// Update updates a URL mapping
func (r *URLRepository) Update(ctx context.Context, mapping *model.URLMapping) error {
	if err := r.db.WithContext(ctx).Save(mapping).Error; err != nil {
		return fmt.Errorf("failed to update URL mapping: %w", err)
	}
	return nil
}

// Delete deletes a URL mapping by short code
func (r *URLRepository) Delete(ctx context.Context, shortCode string) error {
	if err := r.db.WithContext(ctx).Where("short_code = ?", shortCode).Delete(&model.URLMapping{}).Error; err != nil {
		return fmt.Errorf("failed to delete URL mapping: %w", err)
	}
	return nil
}

// Close closes the database connection
func (r *URLRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetDB returns the underlying database instance
func (r *URLRepository) GetDB() *gorm.DB {
	return r.db
}
