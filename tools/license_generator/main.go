package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// استبدل هذا بمفتاحك الخاص (Private Key) الذي زودتك به
const PRIVATE_KEY_HEX = "396f7ae2d0c85834b731df905be07a258bbf75ce87310299180f525e4ca7b2527c29c8f0bb2041bfb51d5590ad7cc5e17d1fb520d3465f5a2891f5d80ad4413e"

func main() {
	var serial string
	var days int

	fmt.Println("--- SASMAN License Generator ---")
	fmt.Print("أدخل رقم السيريال الخاص براوتر العميل: ")
	fmt.Scanln(&serial)
	
	fmt.Print("أدخل عدد أيام الاشتراك (تلقائياً 365): ")
	var daysInput string
	fmt.Scanln(&daysInput)
	
	days = 365 // default
	if daysInput != "" {
		fmt.Sscanf(daysInput, "%d", &days)
	}

	if serial == "" || days <= 0 {
		fmt.Println("خطأ: يرجى إدخال بيانات صحيحة (يجب إدخال رقم السيريال على الأقل)")
		return
	}

	// 1. Decode Private Key
	privKeyBytes, _ := hex.DecodeString(PRIVATE_KEY_HEX)
	privateKey := ed25519.PrivateKey(privKeyBytes)

	// 2. Create Claims
	claims := jwt.MapClaims{
		"serial": serial,
		"exp":    time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
	}

	// 3. Sign Token
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Printf("Error signing token: %v\n", err)
		return
	}

	fmt.Println("\n--------------------------------------------------")
	fmt.Println("تم توليد مفتاح التنشيط بنجاح:")
	fmt.Println(signedToken)
	fmt.Println("--------------------------------------------------")
	fmt.Println("يمكن للعميل الآن لصق هذا الكود في تبويب (تفعيل النظام) داخل لوحة التحكم.")
}
