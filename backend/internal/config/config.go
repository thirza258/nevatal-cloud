package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	Env                string
	AppBaseURL         string
	DatabaseURL        string
	RedisURL           string
	JWTSecret          string
	ProxmoxHost        string
	ProxmoxPort        string
	ProxmoxTokenID     string
	ProxmoxTokenSecret string
	ProxmoxNodeName    string
	ProxmoxStorage     string
	ProxmoxMock        bool
	ProxmoxVerifyTLS   bool
}

func Load() *Config {
	_ = godotenv.Load()

	port := getEnv("PORT", "8090")
	env := getEnv("ENV", "development")
	appBaseURL := getEnv("APP_BASE_URL", "http://localhost:3080")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5435/nevatal_cloud?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "localhost:6385")
	jwtSecret := getEnv("JWT_SECRET", "nevatal-cloud-super-secret-key-production-ready-32-chars")

	pveHost := getEnv("PROXMOX_HOST", "")
	pvePort := getEnv("PROXMOX_PORT", "8006")
	pveTokenID := getEnv("PROXMOX_TOKEN_ID", "")
	pveTokenSecret := getEnv("PROXMOX_TOKEN_SECRET", "")
	pveNode := getEnv("PROXMOX_NODE_NAME", "pve-dell-r640")
	pveStorage := getEnv("PROXMOX_STORAGE", "local-zfs")

	mockStr := getEnv("PROXMOX_MOCK", "")
	isMock := true
	if mockStr == "false" && pveHost != "" && pveTokenID != "" && pveTokenSecret != "" {
		isMock = false
	} else if mockStr == "true" || pveHost == "" {
		isMock = true
	}

	verifyTLS, _ := strconv.ParseBool(getEnv("PROXMOX_VERIFY_TLS", "false"))

	return &Config{
		Port:               port,
		Env:                env,
		AppBaseURL:         appBaseURL,
		DatabaseURL:        dbURL,
		RedisURL:           redisURL,
		JWTSecret:          jwtSecret,
		ProxmoxHost:        pveHost,
		ProxmoxPort:        pvePort,
		ProxmoxTokenID:     pveTokenID,
		ProxmoxTokenSecret: pveTokenSecret,
		ProxmoxNodeName:    pveNode,
		ProxmoxStorage:     pveStorage,
		ProxmoxMock:        isMock,
		ProxmoxVerifyTLS:   verifyTLS,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
