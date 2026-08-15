package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtSecret = []byte(getEnv("SASMAN_JWT_SECRET", "sasman-secret-super-secure-key-2026!"))
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// RateLimiter tracks failed login attempts to prevent brute force attacks
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]int
	locked   map[string]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]int),
		locked:   make(map[string]time.Time),
	}
}

func (rl *RateLimiter) IsLocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if lockUntil, ok := rl.locked[ip]; ok {
		if time.Now().Before(lockUntil) {
			return true
		}
		delete(rl.locked, ip)
		delete(rl.attempts, ip)
	}
	return false
}

func (rl *RateLimiter) RecordFail(ip string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.attempts[ip]++
	if rl.attempts[ip] >= 5 {
		rl.locked[ip] = time.Now().Add(15 * time.Minute)
		return true, 0
	}
	return false, 5 - rl.attempts[ip]
}

func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
	delete(rl.locked, ip)
}

// AuthManager handles authentication, JWT sessions, and password management
type AuthManager struct {
	adminUser     string
	adminPassword string
	limiter       *RateLimiter
	mu            sync.RWMutex
}

func NewAuthManager(defaultUser, defaultPassword string) *AuthManager {
	if defaultUser == "" {
		defaultUser = "admin"
	}
	if defaultPassword == "" {
		defaultPassword = "Mushtaq@Sasman#9977!"
	}
	return &AuthManager{
		adminUser:     defaultUser,
		adminPassword: defaultPassword,
		limiter:       NewRateLimiter(),
	}
}

func (am *AuthManager) GenerateToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 days session
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func (am *AuthManager) VerifyToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	return sub, nil
}

func (am *AuthManager) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Public endpoints that don't need auth
		if path == "/" ||
			path == "/health" ||
			path == "/login" ||
			strings.HasPrefix(path, "/install") ||
			strings.HasPrefix(path, "/download") ||
			path == "/api/login" ||
			path == "/api/logout" ||
			path == "/ws" ||
			strings.HasPrefix(path, "/api/tunnel") ||
			strings.HasPrefix(path, "/api/ota/manifest") ||
			strings.HasPrefix(path, "/api/ota/bin") ||
			path == "/api/ota/report-status" {
			return c.Next()
		}

		// 1. Check Cookie
		cookieToken := c.Cookies("sasman_session")
		if cookieToken != "" {
			if _, err := am.VerifyToken(cookieToken); err == nil {
				return c.Next()
			}
		}

		// 2. Check Authorization Header (Bearer or Basic fallback)
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				if _, err := am.VerifyToken(tokenStr); err == nil {
					return c.Next()
				}
			}
		}

		// If accessing /admin from browser, redirect to /login
		if strings.HasPrefix(path, "/admin") {
			return c.Redirect("/login")
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Unauthorized: Invalid or expired session",
		})
	}
}

func (am *AuthManager) HandleLogin(c *fiber.Ctx) error {
	ip := c.IP()
	if am.limiter.IsLocked(ip) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"error":   "تم قفل الحساب مؤقتاً بسبب تكرار المحاولات الخاطئة. يرجى المحاولة بعد 15 دقيقة.",
		})
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Username == "" {
		if errJson := json.Unmarshal(c.Body(), &req); errJson != nil || req.Username == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "طلب غير صالح: يرجى إدخال اسم المستخدم وكلمة المرور"})
		}
	}

	am.mu.RLock()
	validUser := subtle.ConstantTimeCompare([]byte(req.Username), []byte(am.adminUser)) == 1
	validPass := subtle.ConstantTimeCompare([]byte(req.Password), []byte(am.adminPassword)) == 1
	am.mu.RUnlock()

	if !validUser || !validPass {
		locked, remaining := am.limiter.RecordFail(ip)
		if locked {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"error":   "تم تجاوز عدد المحاولات المسموحة! تم قفل الدخول لـ 15 دقيقة.",
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success":   false,
			"error":     fmt.Sprintf("اسم المستخدم أو كلمة المرور غير صحيحة. المحاولات المتبقية: %d", remaining),
			"remaining": remaining,
		})
	}

	am.limiter.Reset(ip)

	token, err := am.GenerateToken(req.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "فشل إنشاء الجلسة"})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "sasman_session",
		Value:    token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return c.JSON(fiber.Map{
		"success":  true,
		"token":    token,
		"username": req.Username,
	})
}

func (am *AuthManager) HandleChangePassword(c *fiber.Ctx) error {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "كلمة المرور الجديدة مطلوبة"})
	}

	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "يجب أن لا تقل كلمة المرور عن 8 خانات"})
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	if subtle.ConstantTimeCompare([]byte(req.OldPassword), []byte(am.adminPassword)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "error": "كلمة المرور الحالية غير صحيحة"})
	}

	am.adminPassword = req.NewPassword
	return c.JSON(fiber.Map{"success": true, "message": "تم تغيير كلمة المرور بنجاح!"})
}

func (am *AuthManager) HandleLogout(c *fiber.Ctx) error {
	c.ClearCookie("sasman_session")
	return c.JSON(fiber.Map{"success": true, "message": "تم تسجيل الخروج بنجاح"})
}
