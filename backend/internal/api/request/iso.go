package request

import "github.com/nevatal/cloud/internal/usecase"

// UploadISOInput is the multipart form of POST /admin/isos/upload (the file
// itself is the "file" part).
type UploadISOInput struct {
	Name           string `form:"name" binding:"required"`
	Version        string `form:"version" binding:"required"`
	Architecture   string `form:"architecture"`
	Category       string `form:"category"` // Linux, Windows, BSD, Other
	Description    string `form:"description"`
	ProxmoxStorage string `form:"proxmox_storage"`
	Visibility     string `form:"visibility"` // published, unpublished
}

func (r UploadISOInput) ToUsecase() usecase.UploadISOInput {
	return usecase.UploadISOInput(r)
}

// SetISOVisibilityRequest is the body of PATCH /admin/isos/:id/publish.
type SetISOVisibilityRequest = struct {
	Visibility string `json:"visibility" binding:"required"` // published, unpublished
}
