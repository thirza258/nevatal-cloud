package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type ISOService struct {
	db           *gorm.DB
	auditService *AuditService
}

func NewISOService(db *gorm.DB, auditService *AuditService) *ISOService {
	return &ISOService{
		db:           db,
		auditService: auditService,
	}
}

type UploadISOInput struct {
	Name           string `form:"name" binding:"required"`
	Version        string `form:"version" binding:"required"`
	Architecture   string `form:"architecture"`
	Category       string `form:"category"` // Linux, Windows, BSD, Other
	Description    string `form:"description"`
	ProxmoxStorage string `form:"proxmox_storage"`
	Visibility     string `form:"visibility"` // published, unpublished
}

func (s *ISOService) ListPublished() ([]models.OSImage, error) {
	var images []models.OSImage
	err := s.db.Where("visibility = 'published' AND status = 'available'").Order("category asc, name asc").Find(&images).Error
	return images, err
}

func (s *ISOService) ListAllAdmin() ([]models.OSImage, error) {
	var images []models.OSImage
	err := s.db.Order("created_at desc").Find(&images).Error
	return images, err
}

func (s *ISOService) ProcessUpload(adminID uuid.UUID, input UploadISOInput, file *multipart.FileHeader, clientIP string) (*models.OSImage, error) {
	if input.Architecture == "" {
		input.Architecture = "x86_64"
	}
	if input.Category == "" {
		input.Category = "Linux"
	}
	if input.ProxmoxStorage == "" {
		input.ProxmoxStorage = "local"
	}
	if input.Visibility == "" {
		input.Visibility = "published"
	}

	filename := file.Filename
	fileSize := file.Size

	// Calculate SHA256 by streaming from file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, src); err != nil {
		return nil, fmt.Errorf("failed to calculate SHA-256: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	volumeID := fmt.Sprintf("%s:iso/%s", input.ProxmoxStorage, filename)

	now := time.Now()
	img := models.OSImage{
		Name:           input.Name,
		Version:        input.Version,
		Architecture:   input.Architecture,
		Category:       input.Category,
		Description:    input.Description,
		Filename:       filename,
		FileSizeBytes:  fileSize,
		SHA256:         checksum,
		ProxmoxStorage: input.ProxmoxStorage,
		ProxmoxVolume:  volumeID,
		Visibility:     input.Visibility,
		Status:         "available",
		CreatedByID:    &adminID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.db.Create(&img).Error; err != nil {
		return nil, err
	}

	s.auditService.Log(&adminID, "admin.iso.upload", "os_image", &img.ID, "success", clientIP, map[string]interface{}{
		"filename":  filename,
		"sha256":    checksum,
		"file_size": fileSize,
	})

	return &img, nil
}

func (s *ISOService) UpdateVisibility(id uuid.UUID, adminID uuid.UUID, visibility string, clientIP string) error {
	var img models.OSImage
	if err := s.db.First(&img, "id = ?", id).Error; err != nil {
		return errors.New("OS image not found")
	}

	img.Visibility = visibility
	img.UpdatedAt = time.Now()
	if err := s.db.Save(&img).Error; err != nil {
		return err
	}

	s.auditService.Log(&adminID, "admin.iso.visibility_update", "os_image", &img.ID, "success", clientIP, map[string]string{
		"new_visibility": visibility,
	})
	return nil
}

func (s *ISOService) Delete(id uuid.UUID, adminID uuid.UUID, clientIP string) error {
	var img models.OSImage
	if err := s.db.First(&img, "id = ?", id).Error; err != nil {
		return errors.New("OS image not found")
	}

	if err := s.db.Delete(&img).Error; err != nil {
		return err
	}

	s.auditService.Log(&adminID, "admin.iso.delete", "os_image", &id, "success", clientIP, map[string]string{
		"filename": img.Filename,
	})
	return nil
}
