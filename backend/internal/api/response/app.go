package response

import "github.com/nevatal/cloud/internal/models"

// AppCreated is the body of POST /apps: the application under "data" and its
// first deployment beside it (not inside the envelope).
type AppCreated struct {
	Data       *models.Application `json:"data"`
	Deployment *models.Deployment  `json:"deployment"`
}
