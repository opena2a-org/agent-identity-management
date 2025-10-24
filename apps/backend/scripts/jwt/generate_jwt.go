package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func main() {
	userID := uuid.MustParse("7661f186-1de3-4898-bcbd-11bc9490ece7")
	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		fmt.Fprintf(os.Stderr, "Error: JWT_SECRET environment variable is required\n")
		os.Exit(1)
	}

	claims := jwt.MapClaims{
		"user_id":         userID.String(),
		"organization_id": orgID.String(),
		"email":           "admin@opena2a.org",
		"role":            "admin",
		"exp":             time.Now().Add(24 * time.Hour).Unix(),
		"iat":             time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(tokenString)
}
