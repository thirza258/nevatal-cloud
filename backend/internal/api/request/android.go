package request

import "github.com/nevatal/cloud/internal/usecase"

// CreateAndroidInstanceInput is the body of POST /android/instances.
type CreateAndroidInstanceInput struct {
	Name              string `json:"name" binding:"required"`
	ImageID           string `json:"image_id" binding:"required"`
	PlanName          string `json:"plan_name"`
	VCPUs             int    `json:"vcpus"`
	MemoryMB          int    `json:"memory_mb"`
	StorageGB         int    `json:"storage_gb"`
	DisplayResolution string `json:"display_resolution"`
	DisplayDensity    int    `json:"display_density"`
	Orientation       string `json:"orientation"`
	RemoteAccessType  string `json:"remote_access_type"`
	ADBEnabled        bool   `json:"adb_enabled"`
	AutoBackupEnabled bool   `json:"auto_backup_enabled"`
	BackupSchedule    string `json:"backup_schedule"`
}

func (r CreateAndroidInstanceInput) ToUsecase() usecase.CreateAndroidInstanceInput {
	return usecase.CreateAndroidInstanceInput(r)
}

// UpdateAndroidInstanceInput is the body of PATCH /android/instances/:id.
type UpdateAndroidInstanceInput struct {
	Name              *string `json:"name"`
	DisplayResolution *string `json:"display_resolution"`
	DisplayDensity    *int    `json:"display_density"`
	Orientation       *string `json:"orientation"`
	AutoBackupEnabled *bool   `json:"auto_backup_enabled"`
	BackupSchedule    *string `json:"backup_schedule"`
	ADBEnabled        *bool   `json:"adb_enabled"`
}

func (r UpdateAndroidInstanceInput) ToUsecase() usecase.UpdateAndroidInstanceInput {
	return usecase.UpdateAndroidInstanceInput(r)
}

// InstallAPKInput is the body of POST /android/instances/:id/applications.
type InstallAPKInput struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name" binding:"required"`
	VersionName string `json:"version_name"`
	APKURL      string `json:"apk_url"`
	FileBytes   int64  `json:"file_bytes"`
}

func (r InstallAPKInput) ToUsecase() usecase.InstallAPKInput {
	return usecase.InstallAPKInput(r)
}

// CreateAndroidBackupRequest is the optional body of
// POST /android/instances/:id/backups.
type CreateAndroidBackupRequest = struct {
	BackupName string `json:"backup_name"`
}

// CreateAndroidImageInput is the body of POST /admin/android/images.
type CreateAndroidImageInput struct {
	Name           string `json:"name" binding:"required"`
	Slug           string `json:"slug" binding:"required"`
	AndroidVersion string `json:"android_version" binding:"required"`
	Architecture   string `json:"architecture"`
	ImageType      string `json:"image_type"`
	SystemImageURL string `json:"system_image_url" binding:"required"`
	VendorImageURL string `json:"vendor_image_url" binding:"required"`
	ImageSource    string `json:"image_source"`
	ChecksumSHA256 string `json:"checksum_sha256" binding:"required"`
	MinCPU         int    `json:"min_cpu"`
	MinRAMMB       int    `json:"min_ram_mb"`
	MinStorageGB   int    `json:"min_storage_gb"`
	Description    string `json:"description"`
}

func (r CreateAndroidImageInput) ToUsecase() usecase.CreateAndroidImageInput {
	return usecase.CreateAndroidImageInput(r)
}

// UpdateAndroidImageInput is the body of PUT /admin/android/images/:id.
type UpdateAndroidImageInput struct {
	Name           *string `json:"name"`
	AndroidVersion *string `json:"android_version"`
	ImageType      *string `json:"image_type"`
	Description    *string `json:"description"`
	MinCPU         *int    `json:"min_cpu"`
	MinRAMMB       *int    `json:"min_ram_mb"`
	MinStorageGB   *int    `json:"min_storage_gb"`
}

func (r UpdateAndroidImageInput) ToUsecase() usecase.UpdateAndroidImageInput {
	return usecase.UpdateAndroidImageInput(r)
}
