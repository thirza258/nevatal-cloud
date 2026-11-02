package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AppRepository persists deployed applications with their deployments and
// environment variables. Applications are soft-deleted through their own
// deleted_at column.
type AppRepository interface {
	// List returns live applications (newest first). Only one deployment,
	// the newest across the result, is preloaded (preload LIMIT semantics).
	List(ctx context.Context, ownerID *uuid.UUID) ([]models.Application, error)
	// FindByID returns a live application with deployments (newest first)
	// and environment variables preloaded.
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Application, error)
	CountBySubdomain(ctx context.Context, subdomain string) (int64, error)
	Create(ctx context.Context, app *models.Application) error
	// UpdateStatus sets the status of app.
	UpdateStatus(ctx context.Context, app *models.Application, status string) error
	// SoftDelete sets deleted_at on app.
	SoftDelete(ctx context.Context, app *models.Application, at *time.Time) error

	FindDeployment(ctx context.Context, appID, deploymentID uuid.UUID) (*models.Deployment, error)
	CreateDeployment(ctx context.Context, dep *models.Deployment) error

	ListEnvVars(ctx context.Context, appID uuid.UUID) ([]models.ApplicationEnvVar, error)
	FindEnvVar(ctx context.Context, appID uuid.UUID, key string) (*models.ApplicationEnvVar, error)
	CreateEnvVar(ctx context.Context, env *models.ApplicationEnvVar) error
	SaveEnvVar(ctx context.Context, env *models.ApplicationEnvVar) error
	DeleteEnvVar(ctx context.Context, appID uuid.UUID, key string) error
}

type appRepository struct{ base }

var _ AppRepository = (*appRepository)(nil)

func NewAppRepository(db *gorm.DB) AppRepository {
	return &appRepository{base{db}}
}

func (r *appRepository) List(ctx context.Context, ownerID *uuid.UUID) ([]models.Application, error) {
	var apps []models.Application
	// A Limit inside Preload applies to the single IN (...) query, not per
	// app, so load newest-first and keep the latest deployment of each app.
	query := r.conn(ctx).Preload("Deployments", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Where("deleted_at IS NULL")

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	err := query.Order("created_at desc").Find(&apps).Error
	for i := range apps {
		if len(apps[i].Deployments) > 1 {
			apps[i].Deployments = apps[i].Deployments[:1]
		}
	}
	return apps, err
}

func (r *appRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Application, error) {
	var app models.Application
	query := r.conn(ctx).Preload("Deployments", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Preload("EnvVars").Where("id = ? AND deleted_at IS NULL", id)

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	if err := query.First(&app).Error; err != nil {
		return nil, mapErr(err)
	}
	return &app, nil
}

func (r *appRepository) CountBySubdomain(ctx context.Context, subdomain string) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.Application{}).Where("assigned_subdomain = ? AND deleted_at IS NULL", subdomain).Count(&count).Error
	return count, err
}

func (r *appRepository) Create(ctx context.Context, app *models.Application) error {
	return r.conn(ctx).Create(app).Error
}

func (r *appRepository) UpdateStatus(ctx context.Context, app *models.Application, status string) error {
	return r.conn(ctx).Model(app).Update("status", status).Error
}

func (r *appRepository) SoftDelete(ctx context.Context, app *models.Application, at *time.Time) error {
	return r.conn(ctx).Model(app).Update("deleted_at", at).Error
}

func (r *appRepository) FindDeployment(ctx context.Context, appID, deploymentID uuid.UUID) (*models.Deployment, error) {
	var dep models.Deployment
	if err := r.conn(ctx).Where("id = ? AND application_id = ?", deploymentID, appID).First(&dep).Error; err != nil {
		return nil, mapErr(err)
	}
	return &dep, nil
}

func (r *appRepository) CreateDeployment(ctx context.Context, dep *models.Deployment) error {
	return r.conn(ctx).Create(dep).Error
}

func (r *appRepository) ListEnvVars(ctx context.Context, appID uuid.UUID) ([]models.ApplicationEnvVar, error) {
	var envs []models.ApplicationEnvVar
	err := r.conn(ctx).Where("application_id = ?", appID).Find(&envs).Error
	return envs, err
}

func (r *appRepository) FindEnvVar(ctx context.Context, appID uuid.UUID, key string) (*models.ApplicationEnvVar, error) {
	var env models.ApplicationEnvVar
	if err := r.conn(ctx).Where("application_id = ? AND key = ?", appID, key).First(&env).Error; err != nil {
		return nil, mapErr(err)
	}
	return &env, nil
}

func (r *appRepository) CreateEnvVar(ctx context.Context, env *models.ApplicationEnvVar) error {
	return r.conn(ctx).Create(env).Error
}

func (r *appRepository) SaveEnvVar(ctx context.Context, env *models.ApplicationEnvVar) error {
	return r.conn(ctx).Save(env).Error
}

func (r *appRepository) DeleteEnvVar(ctx context.Context, appID uuid.UUID, key string) error {
	return r.conn(ctx).Where("application_id = ? AND key = ?", appID, key).Delete(&models.ApplicationEnvVar{}).Error
}
