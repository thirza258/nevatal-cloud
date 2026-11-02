package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// ISORepository persists installable OS images (os_images).
type ISORepository interface {
	// ListPublished returns published, available images by category and name.
	ListPublished(ctx context.Context) ([]models.OSImage, error)
	// ListAll returns every image, newest first.
	ListAll(ctx context.Context) ([]models.OSImage, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.OSImage, error)
	Create(ctx context.Context, img *models.OSImage) error
	Save(ctx context.Context, img *models.OSImage) error
	Delete(ctx context.Context, img *models.OSImage) error
}

type isoRepository struct{ base }

var _ ISORepository = (*isoRepository)(nil)

func NewISORepository(db *gorm.DB) ISORepository {
	return &isoRepository{base{db}}
}

func (r *isoRepository) ListPublished(ctx context.Context) ([]models.OSImage, error) {
	var images []models.OSImage
	err := r.conn(ctx).Where("visibility = 'published' AND status = 'available'").Order("category asc, name asc").Find(&images).Error
	return images, err
}

func (r *isoRepository) ListAll(ctx context.Context) ([]models.OSImage, error) {
	var images []models.OSImage
	err := r.conn(ctx).Order("created_at desc").Find(&images).Error
	return images, err
}

func (r *isoRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.OSImage, error) {
	var img models.OSImage
	if err := r.conn(ctx).First(&img, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &img, nil
}

func (r *isoRepository) Create(ctx context.Context, img *models.OSImage) error {
	return r.conn(ctx).Create(img).Error
}

func (r *isoRepository) Save(ctx context.Context, img *models.OSImage) error {
	return r.conn(ctx).Save(img).Error
}

func (r *isoRepository) Delete(ctx context.Context, img *models.OSImage) error {
	return r.conn(ctx).Delete(img).Error
}
