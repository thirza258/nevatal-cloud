package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bootstrapLockKey serializes migrations and seeding across API replicas that
// start at the same time (Kubernetes rollouts, docker compose --scale).
const bootstrapLockKey int64 = 0x4e56544c_00000001

// withBootstrapLock runs fn while holding a session-level Postgres advisory
// lock on a dedicated connection.
func withBootstrapLock(ctx context.Context, db *gorm.DB, fn func() error) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("bootstrap lock: %w", err)
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap lock: open connection: %w", err)
	}
	defer conn.Close()

	start := time.Now()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", bootstrapLockKey); err != nil {
		return fmt.Errorf("bootstrap lock: acquire: %w", err)
	}
	if waited := time.Since(start); waited > time.Second {
		log.Printf("Waited %s for another replica to finish migrations/seeding.", waited.Round(time.Millisecond))
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", bootstrapLockKey); err != nil {
			log.Printf("Warning: releasing bootstrap lock failed: %v", err)
		}
	}()

	return fn()
}

// seedAdmins creates the administrator accounts for the active mode.
//
//   - DEMO_MODE=true: the public demo admin (shown in the UI role switcher),
//     plus ADMIN_EMAIL/ADMIN_PASSWORD when both are set.
//   - DEMO_MODE=false: only ADMIN_EMAIL/ADMIN_PASSWORD. Public demo accounts
//     left over from a demo database are suspended.
func seedAdmins(db *gorm.DB, cfg *config.Config) error {
	if cfg.DemoMode {
		if err := ensureAdmin(db, config.DemoAdminEmail, config.DemoAdminPassword, false); err != nil {
			return err
		}
		if cfg.AdminEmail != "" && cfg.AdminPassword != "" && cfg.AdminEmail != config.DemoAdminEmail {
			return ensureAdmin(db, cfg.AdminEmail, cfg.AdminPassword, true)
		}
		return nil
	}

	email, password := cfg.AdminEmail, cfg.AdminPassword
	if email == "" || password == "" {
		if cfg.IsProduction() {
			return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD are required when DEMO_MODE=false")
		}
		email, password = config.DemoAdminEmail, config.DemoAdminPassword
		log.Printf("WARNING: ADMIN_EMAIL/ADMIN_PASSWORD not set; using the public development admin %s (development only).", email)
	}

	if err := ensureAdmin(db, email, password, true); err != nil {
		return err
	}
	if email != config.DemoAdminEmail || password != config.DemoAdminPassword {
		suspendPublicDemoAccounts(db, email)
	}
	return nil
}

// ensureAdmin creates the account when missing. An existing account is made an
// active administrator; with rotatePublic, a password that is still one of the
// published demo passwords is replaced by the configured one. Passwords the
// operator changed later are never overwritten.
func ensureAdmin(db *gorm.DB, email, password string, rotatePublic bool) error {
	var user models.User
	err := db.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash admin password: %w", err)
		}
		now := time.Now()
		user = models.User{
			Email:           email,
			PasswordHash:    string(hash),
			DisplayName:     "Platform Administrator",
			Role:            models.RoleAdmin,
			Status:          models.UserStatusActive,
			EmailVerifiedAt: &now,
		}
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&user).Error; err != nil {
				return fmt.Errorf("create admin %s: %w", email, err)
			}
			quota := models.ResourceQuota{
				UserID:       user.ID,
				MaxVCPUs:     32,
				MaxMemoryMB:  65536,
				MaxStorageGB: 1000,
				MaxInstances: 20,
			}
			if err := tx.Create(&quota).Error; err != nil {
				return fmt.Errorf("create admin quota: %w", err)
			}
			log.Printf("Seeded administrator account: %s", email)
			return nil
		})
	}
	if err != nil {
		return fmt.Errorf("look up admin %s: %w", email, err)
	}

	updates := map[string]any{}
	if user.Role != models.RoleAdmin {
		updates["role"] = models.RoleAdmin
	}
	if user.Status != models.UserStatusActive {
		updates["status"] = models.UserStatusActive
	}
	if rotatePublic && usesPublicDemoPassword(user.PasswordHash) &&
		password != config.DemoAdminPassword && password != config.DemoUserPassword {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash admin password: %w", err)
		}
		updates["password_hash"] = string(hash)
		log.Printf("SECURITY: %s still used a public demo password; replaced it with ADMIN_PASSWORD.", email)
	}
	if len(updates) == 0 {
		return nil
	}
	if err := db.Model(&user).Updates(updates).Error; err != nil {
		return fmt.Errorf("update admin %s: %w", email, err)
	}
	return nil
}

func usesPublicDemoPassword(hash string) bool {
	for _, pw := range []string{config.DemoAdminPassword, config.DemoUserPassword} {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil {
			return true
		}
	}
	return false
}

// suspendPublicDemoAccounts locks demo accounts whose credentials are public.
// Their existing JWTs stay valid until expiry, so production must also use a
// fresh JWT_SECRET (see docs/PROXMOX_INSTALL.md).
func suspendPublicDemoAccounts(db *gorm.DB, keepEmail string) {
	accounts := []struct{ email, password string }{
		{config.DemoAdminEmail, config.DemoAdminPassword},
		{config.DemoUserEmail, config.DemoUserPassword},
	}
	for _, acct := range accounts {
		if acct.email == keepEmail {
			continue
		}
		var user models.User
		if err := db.Where("email = ?", acct.email).First(&user).Error; err != nil {
			continue
		}
		if user.Status == models.UserStatusSuspended {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(acct.password)) != nil {
			continue
		}
		if err := db.Model(&user).Update("status", models.UserStatusSuspended).Error; err != nil {
			log.Printf("Warning: could not suspend public demo account %s: %v", acct.email, err)
			continue
		}
		log.Printf("SECURITY: suspended %s because it still uses the public demo password (DEMO_MODE=false).", acct.email)
	}
}
