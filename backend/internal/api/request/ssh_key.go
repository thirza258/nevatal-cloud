package request

// CreateSSHKeyRequest is the body of POST /ssh-keys.
type CreateSSHKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
}

// RenameSSHKeyRequest is the body of PATCH /ssh-keys/:id.
type RenameSSHKeyRequest struct {
	Name string `json:"name" binding:"required"`
}
