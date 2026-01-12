package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"unwise-backend/config"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.SupabaseJWTSecret == "" {
		log.Fatalf("SUPABASE_JWT_SECRET not found in .env")
	}

	userID := "d5a2089c-e39a-4b62-a973-778f6729323d"

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iss":  "supabase",
		"aud":  "authenticated",
		"role": "authenticated",
	})

	var secret []byte
	decoded, err := base64.StdEncoding.DecodeString(cfg.SupabaseJWTSecret)
	if err == nil {
		secret = decoded
	} else {
		secret = []byte(cfg.SupabaseJWTSecret)
	}

	tokenString, err := token.SignedString(secret)
	if err != nil {
		log.Fatalf("Failed to sign token: %v", err)
	}

	fmt.Println("Generated Token for Alice (alice@example.com):")
	fmt.Println("-----------------------------------------------")
	fmt.Println(tokenString)
	fmt.Println("-----------------------------------------------")
	fmt.Println("\nUser IDs for Postman Variables:")
	fmt.Println("user1Id (Alice):   d5a2089c-e39a-4b62-a973-778f6729323d")
	fmt.Println("user2Id (Bob):     38c072a2-43f9-42b9-b603-6061c49d5c2d")
	fmt.Println("user3Id (Charlie): ad655801-23a9-4a33-8695-81d4426604fb")
	fmt.Println("user4Id (Diana):   0cc055a7-860a-4ac9-8018-82380ba204a3")
}
