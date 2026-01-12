package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                      string
	Env                       string
	DatabaseURL               string
	SupabaseURL               string
	SupabaseJWTSecret         string
	SupabaseServiceRoleKey    string
	GeminiAPIKey              string
	SupabaseStorageBucket     string
	SupabaseStorageURL        string
	SupabaseGroupPhotosBucket string
	SupabaseUserAvatarsBucket string
	AllowedOrigins            []string
	MaxBodySize               int64 
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	env := getEnv("ENV", "development")
	origins := os.Getenv("ALLOWED_ORIGINS")
	var allowedOrigins []string
	if origins != "" {
		allowedOrigins = splitOrigins(origins)
	} else {
		if env == "production" {
			log.Println("[WARNING] ALLOWED_ORIGINS not set in production! Defaulting to '*' which is insecure.")
			log.Println("[WARNING] Set ALLOWED_ORIGINS to your frontend URL(s), e.g., 'https://your-app.vercel.app'")
		}
		allowedOrigins = []string{"*"}
	}

	maxBodySize := int64(1 * 1024 * 1024) 
	if sizeStr := os.Getenv("MAX_BODY_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			maxBodySize = size
		}
	}

	return &Config{
		Port:                      getEnv("PORT", "8080"),
		Env:                       env,
		DatabaseURL:               getEnv("DATABASE_URL", ""),
		SupabaseURL:               getEnv("SUPABASE_URL", ""),
		SupabaseJWTSecret:         getEnv("SUPABASE_JWT_SECRET", ""),
		SupabaseServiceRoleKey:    getEnv("SUPABASE_SERVICE_ROLE_KEY", ""),
		GeminiAPIKey:              getEnv("GEMINI_API_KEY", ""),
		SupabaseStorageBucket:     getEnv("SUPABASE_STORAGE_BUCKET", "receipts"),
		SupabaseStorageURL:        getEnv("SUPABASE_STORAGE_URL", ""),
		SupabaseGroupPhotosBucket: getEnv("SUPABASE_GROUP_PHOTOS_BUCKET", "group-photos"),
		SupabaseUserAvatarsBucket: getEnv("SUPABASE_USER_AVATARS_BUCKET", "user-avatars"),
		AllowedOrigins:            allowedOrigins,
		MaxBodySize:               maxBodySize,
	}, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func splitOrigins(origins string) []string {
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
