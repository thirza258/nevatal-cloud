package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AndroidRepository persists the Waydroid Android cloud: instances with their
// resources, data volumes, applications, sessions and backups, the worker
// nodes they run on, and the admin image registry.
type AndroidRepository interface {
	// Instances
	// List returns instances (newest first) with image, worker node,
	// applications and backups preloaded, optionally filtered by owner and
	// status.
	List(ctx context.Context, ownerID *uuid.UUID, status string) ([]models.AndroidInstance, error)
	// FindByID returns an instance with every association preloaded. With
	// ownerID set, other owners' instances are not found.
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.AndroidInstance, error)
	CreateInstance(ctx context.Context, inst *models.AndroidInstance) error
	// UpdateInstance writes the given columns of inst.
	UpdateInstance(ctx context.Context, inst *models.AndroidInstance, values map[string]interface{}) error
	// SetLastBackup sets last_backup_at on inst.
	SetLastBackup(ctx context.Context, inst *models.AndroidInstance, at *time.Time) error
	CreateResource(ctx context.Context, res *models.AndroidInstanceResource) error
	CreateDataVolume(ctx context.Context, vol *models.AndroidDataVolume) error

	// Worker nodes
	// FindReadyWorker returns the ready worker with the fewest instances.
	FindReadyWorker(ctx context.Context) (*models.AndroidWorkerNode, error)
	CreateWorker(ctx context.Context, worker *models.AndroidWorkerNode) error
	IncrementWorkerInstances(ctx context.Context, worker *models.AndroidWorkerNode) error
	// DecrementWorkerInstances lowers the instance count of a worker, never
	// below zero.
	DecrementWorkerInstances(ctx context.Context, workerID uuid.UUID) error

	// Applications
	// ListApplications returns an instance's applications, newest first.
	ListApplications(ctx context.Context, instanceID uuid.UUID) ([]models.AndroidApplication, error)
	FindApplication(ctx context.Context, instanceID uuid.UUID, packageName string) (*models.AndroidApplication, error)
	CreateApplication(ctx context.Context, app *models.AndroidApplication) error
	DeleteApplication(ctx context.Context, app *models.AndroidApplication) error

	// Sessions
	// ListSessions returns an instance's sessions, newest first.
	ListSessions(ctx context.Context, instanceID uuid.UUID) ([]models.AndroidSession, error)
	CountActiveSessions(ctx context.Context, instanceID uuid.UUID) (int64, error)
	CreateSession(ctx context.Context, session *models.AndroidSession) error
	// EndSession marks one session of the instance inactive.
	EndSession(ctx context.Context, instanceID, sessionID uuid.UUID, at *time.Time) error

	// Backups
	// ListBackups returns an instance's backups, newest first.
	ListBackups(ctx context.Context, instanceID uuid.UUID) ([]models.AndroidBackup, error)
	FindBackup(ctx context.Context, instanceID, backupID uuid.UUID) (*models.AndroidBackup, error)
	CreateBackup(ctx context.Context, backup *models.AndroidBackup) error
	DeleteBackup(ctx context.Context, instanceID, backupID uuid.UUID) error

	// Image registry
	// ListImages returns images (newest Android version first) with their
	// versions preloaded, optionally only published ones.
	ListImages(ctx context.Context, publishedOnly bool) ([]models.AndroidImage, error)
	// FindImage returns an image with its versions preloaded.
	FindImage(ctx context.Context, id uuid.UUID) (*models.AndroidImage, error)
	FindPublishedImage(ctx context.Context, id uuid.UUID) (*models.AndroidImage, error)
	FindImageBySlug(ctx context.Context, slug string) (*models.AndroidImage, error)
	CreateImage(ctx context.Context, image *models.AndroidImage) error
	CreateImageVersion(ctx context.Context, version *models.AndroidImageVersion) error
	// UpdateImage writes the given columns of image.
	UpdateImage(ctx context.Context, image *models.AndroidImage, values map[string]interface{}) error
	// SetImageFlag sets one boolean column of image.
	SetImageFlag(ctx context.Context, image *models.AndroidImage, column string, value bool) error
	DeleteImage(ctx context.Context, id uuid.UUID) error
}

type androidRepository struct{ base }

var _ AndroidRepository = (*androidRepository)(nil)

func NewAndroidRepository(db *gorm.DB) AndroidRepository {
	return &androidRepository{base{db}}
}

func (r *androidRepository) List(ctx context.Context, ownerID *uuid.UUID, status string) ([]models.AndroidInstance, error) {
	var instances []models.AndroidInstance
	query := r.conn(ctx).Model(&models.AndroidInstance{}).
		Preload("Image").
		Preload("WorkerNode").
		Preload("Applications").
		Preload("Backups").
		Order("created_at desc")

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Find(&instances).Error
	return instances, err
}

func (r *androidRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.AndroidInstance, error) {
	var instance models.AndroidInstance
	query := r.conn(ctx).Model(&models.AndroidInstance{}).
		Preload("Image").
		Preload("WorkerNode").
		Preload("Resources").
		Preload("Applications").
		Preload("Backups").
		Preload("Sessions").
		Where("id = ?", id)

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	if err := query.First(&instance).Error; err != nil {
		return nil, mapErr(err)
	}
	return &instance, nil
}

func (r *androidRepository) CreateInstance(ctx context.Context, inst *models.AndroidInstance) error {
	return r.conn(ctx).Create(inst).Error
}

func (r *androidRepository) UpdateInstance(ctx context.Context, inst *models.AndroidInstance, values map[string]interface{}) error {
	return r.conn(ctx).Model(inst).Updates(values).Error
}

func (r *androidRepository) SetLastBackup(ctx context.Context, inst *models.AndroidInstance, at *time.Time) error {
	return r.conn(ctx).Model(inst).Update("last_backup_at", at).Error
}

func (r *androidRepository) CreateResource(ctx context.Context, res *models.AndroidInstanceResource) error {
	return r.conn(ctx).Create(res).Error
}

func (r *androidRepository) CreateDataVolume(ctx context.Context, vol *models.AndroidDataVolume) error {
	return r.conn(ctx).Create(vol).Error
}

func (r *androidRepository) FindReadyWorker(ctx context.Context) (*models.AndroidWorkerNode, error) {
	var worker models.AndroidWorkerNode
	if err := r.conn(ctx).Where("status = ?", "ready").Order("active_instances_count asc").First(&worker).Error; err != nil {
		return nil, mapErr(err)
	}
	return &worker, nil
}

func (r *androidRepository) CreateWorker(ctx context.Context, worker *models.AndroidWorkerNode) error {
	return r.conn(ctx).Create(worker).Error
}

func (r *androidRepository) IncrementWorkerInstances(ctx context.Context, worker *models.AndroidWorkerNode) error {
	return r.conn(ctx).Model(worker).UpdateColumn("active_instances_count", gorm.Expr("active_instances_count + 1")).Error
}

func (r *androidRepository) DecrementWorkerInstances(ctx context.Context, workerID uuid.UUID) error {
	return r.conn(ctx).Model(&models.AndroidWorkerNode{}).
		Where("id = ?", workerID).
		UpdateColumn("active_instances_count", gorm.Expr("GREATEST(0, active_instances_count - 1)")).Error
}

func (r *androidRepository) ListApplications(ctx context.Context, instanceID uuid.UUID) ([]models.AndroidApplication, error) {
	var apps []models.AndroidApplication
	err := r.conn(ctx).Where("instance_id = ?", instanceID).Order("installed_at desc").Find(&apps).Error
	return apps, err
}

func (r *androidRepository) FindApplication(ctx context.Context, instanceID uuid.UUID, packageName string) (*models.AndroidApplication, error) {
	var app models.AndroidApplication
	if err := r.conn(ctx).Where("instance_id = ? AND package_name = ?", instanceID, packageName).First(&app).Error; err != nil {
		return nil, mapErr(err)
	}
	return &app, nil
}

func (r *androidRepository) CreateApplication(ctx context.Context, app *models.AndroidApplication) error {
	return r.conn(ctx).Create(app).Error
}

func (r *androidRepository) DeleteApplication(ctx context.Context, app *models.AndroidApplication) error {
	return r.conn(ctx).Delete(app).Error
}

func (r *androidRepository) ListSessions(ctx context.Context, instanceID uuid.UUID) ([]models.AndroidSession, error) {
	var sessions []models.AndroidSession
	err := r.conn(ctx).Where("instance_id = ?", instanceID).Order("started_at desc").Find(&sessions).Error
	return sessions, err
}

func (r *androidRepository) CountActiveSessions(ctx context.Context, instanceID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.AndroidSession{}).Where("instance_id = ? AND is_active = ?", instanceID, true).Count(&n).Error
	return n, err
}

func (r *androidRepository) CreateSession(ctx context.Context, session *models.AndroidSession) error {
	return r.conn(ctx).Create(session).Error
}

func (r *androidRepository) EndSession(ctx context.Context, instanceID, sessionID uuid.UUID, at *time.Time) error {
	return r.conn(ctx).Model(&models.AndroidSession{}).
		Where("id = ? AND instance_id = ?", sessionID, instanceID).
		Updates(map[string]interface{}{
			"is_active": false,
			"ended_at":  at,
		}).Error
}

func (r *androidRepository) ListBackups(ctx context.Context, instanceID uuid.UUID) ([]models.AndroidBackup, error) {
	var backups []models.AndroidBackup
	err := r.conn(ctx).Where("instance_id = ?", instanceID).Order("created_at desc").Find(&backups).Error
	return backups, err
}

func (r *androidRepository) FindBackup(ctx context.Context, instanceID, backupID uuid.UUID) (*models.AndroidBackup, error) {
	var backup models.AndroidBackup
	if err := r.conn(ctx).Where("id = ? AND instance_id = ?", backupID, instanceID).First(&backup).Error; err != nil {
		return nil, mapErr(err)
	}
	return &backup, nil
}

func (r *androidRepository) CreateBackup(ctx context.Context, backup *models.AndroidBackup) error {
	return r.conn(ctx).Create(backup).Error
}

func (r *androidRepository) DeleteBackup(ctx context.Context, instanceID, backupID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND instance_id = ?", backupID, instanceID).Delete(&models.AndroidBackup{}).Error
}

func (r *androidRepository) ListImages(ctx context.Context, publishedOnly bool) ([]models.AndroidImage, error) {
	var images []models.AndroidImage
	query := r.conn(ctx).Model(&models.AndroidImage{}).Preload("Versions").Order("android_version desc")
	if publishedOnly {
		query = query.Where("is_published = ?", true)
	}
	err := query.Find(&images).Error
	return images, err
}

func (r *androidRepository) FindImage(ctx context.Context, id uuid.UUID) (*models.AndroidImage, error) {
	var image models.AndroidImage
	if err := r.conn(ctx).Model(&models.AndroidImage{}).Preload("Versions").Where("id = ?", id).First(&image).Error; err != nil {
		return nil, mapErr(err)
	}
	return &image, nil
}

func (r *androidRepository) FindPublishedImage(ctx context.Context, id uuid.UUID) (*models.AndroidImage, error) {
	var image models.AndroidImage
	if err := r.conn(ctx).Where("id = ? AND is_published = ?", id, true).First(&image).Error; err != nil {
		return nil, mapErr(err)
	}
	return &image, nil
}

func (r *androidRepository) CreateImage(ctx context.Context, image *models.AndroidImage) error {
	return r.conn(ctx).Create(image).Error
}

func (r *androidRepository) CreateImageVersion(ctx context.Context, version *models.AndroidImageVersion) error {
	return r.conn(ctx).Create(version).Error
}

func (r *androidRepository) UpdateImage(ctx context.Context, image *models.AndroidImage, values map[string]interface{}) error {
	return r.conn(ctx).Model(image).Updates(values).Error
}

func (r *androidRepository) SetImageFlag(ctx context.Context, image *models.AndroidImage, column string, value bool) error {
	return r.conn(ctx).Model(image).Update(column, value).Error
}

func (r *androidRepository) DeleteImage(ctx context.Context, id uuid.UUID) error {
	return r.conn(ctx).Where("id = ?", id).Delete(&models.AndroidImage{}).Error
}

func (r *androidRepository) FindImageBySlug(ctx context.Context, slug string) (*models.AndroidImage, error) {
	var img models.AndroidImage
	if err := r.conn(ctx).First(&img, "slug = ?", slug).Error; err != nil {
		return nil, mapErr(err)
	}
	return &img, nil
}
