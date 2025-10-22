package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$yybTFh5z/GHzwIHl/bNotOCVU3L9IxS/A0ufCwLiPbhFp4/DiYtsu"
	password := "AIM2025!Secure"

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		fmt.Println("❌ Password does NOT match hash:", err)
	} else {
		fmt.Println("✅ Password matches hash!")
	}
}
