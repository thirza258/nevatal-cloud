package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

type ISOUsecase struct {
	isos         repository.ISORepository
	auditService *AuditUsecase
}

func NewISOUsecase(isos repository.ISORepository, auditService *AuditUsecase) *ISOUsecase {
	return &ISOUsecase{
		isos:         isos,
		auditService: auditService,
	}
}

type UploadISOInput struct {
	Name           string
	Version        string
	Architecture   string
	Category       string // Linux, Windows, BSD, Other
	Description    string
	ProxmoxStorage string
	Visibility     string // published, unpublished
}

func (s *ISOUsecase) ListPublished(ctx context.Context) ([]models.OSImage, error) {
	return s.isos.ListPublished(ctx)
}

func (s *ISOUsecase) ListAllAdmin(ctx context.Context) ([]models.OSImage, error) {
	return s.isos.ListAll(ctx)
}

func (s *ISOUsecase) ProcessUpload(ctx context.Context, adminID uuid.UUID, input UploadISOInput, file *multipart.FileHeader, clientIP string) (*models.OSImage, error) {
	ctx = context.WithoutCancel(ctx)

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

	if err := s.isos.Create(ctx, &img); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &adminID, "admin.iso.upload", "os_image", &img.ID, "success", clientIP, map[string]interface{}{
		"filename":  filename,
		"sha256":    checksum,
		"file_size": fileSize,
	})

	return &img, nil
}

func (s *ISOUsecase) UpdateVisibility(ctx context.Context, id uuid.UUID, adminID uuid.UUID, visibility string, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	img, err := s.isos.FindByID(ctx, id)
	if err != nil {
		return errors.New("OS image not found")
	}

	img.Visibility = visibility
	img.UpdatedAt = time.Now()
	if err := s.isos.Save(ctx, img); err != nil {
		return err
	}

	s.auditService.Log(ctx, &adminID, "admin.iso.visibility_update", "os_image", &img.ID, "success", clientIP, map[string]string{
		"new_visibility": visibility,
	})
	return nil
}

func (s *ISOUsecase) Delete(ctx context.Context, id uuid.UUID, adminID uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	img, err := s.isos.FindByID(ctx, id)
	if err != nil {
		return errors.New("OS image not found")
	}

	if err := s.isos.Delete(ctx, img); err != nil {
		return err
	}

	s.auditService.Log(ctx, &adminID, "admin.iso.delete", "os_image", &id, "success", clientIP, map[string]string{
		"filename": img.Filename,
	})
	return nil
}
