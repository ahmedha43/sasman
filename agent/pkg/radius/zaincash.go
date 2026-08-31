package radius

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ZainCashAgentConfig struct {
	MerchantID string
	Secret     string
	MSISDN     string
	BaseURL    string
}

type AgentZainCashService struct {
	cfg ZainCashAgentConfig
}

func NewAgentZainCashService() *AgentZainCashService {
	merchantID := os.Getenv("ZAINCASH_MERCHANT_ID")
	if merchantID == "" {
		merchantID = "9f0937eeaa4a44068f703c09cf4669a6"
	}
	msisdn := os.Getenv("ZAINCASH_MSISDN")
	if msisdn == "" {
		msisdn = "9647819597948"
	}
	secret := os.Getenv("ZAINCASH_SECRET")
	if secret == "" {
		secret = "m82U5S7FIbRZ1sqB2LSg2ukyyyf9x26a"
	}
	baseURL := os.Getenv("ZAINCASH_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.zaincash.iq"
	}

	return &AgentZainCashService{
		cfg: ZainCashAgentConfig{
			MerchantID: merchantID,
			Secret:     secret,
			MSISDN:     msisdn,
			BaseURL:    strings.TrimSuffix(baseURL, "/"),
		},
	}
}

func (s *AgentZainCashService) GenerateJWT(claims map[string]interface{}) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	unsignedToken := fmt.Sprintf("%s.%s", headerB64, claimsB64)

	h := hmac.New(sha256.New, []byte(s.cfg.Secret))
	h.Write([]byte(unsignedToken))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s.%s", unsignedToken, signature), nil
}

func (s *AgentZainCashService) VerifyJWT(tokenStr string) (map[string]interface{}, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	unsignedToken := fmt.Sprintf("%s.%s", parts[0], parts[1])
	h := hmac.New(sha256.New, []byte(s.cfg.Secret))
	h.Write([]byte(unsignedToken))
	expectedSignature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if expectedSignature != parts[2] {
		return nil, fmt.Errorf("invalid token signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	return claims, nil
}

type AgentCreateTxReq struct {
	Amount      int
	ServiceName string
	OrderID     string
	RedirectURL string
}

func (s *AgentZainCashService) CreateTransaction(req AgentCreateTxReq) (string, error) {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"amount":      req.Amount,
		"serviceType": req.ServiceName,
		"msisdn":      s.cfg.MSISDN,
		"orderId":     req.OrderID,
		"redirectUrl": req.RedirectURL,
		"iat":         now,
		"exp":         now + 4*3600,
	}

	token, err := s.GenerateJWT(claims)
	if err != nil {
		return "", fmt.Errorf("generate jwt: %w", err)
	}

	formValues := url.Values{}
	formValues.Set("token", token)
	formValues.Set("merchantId", s.cfg.MerchantID)
	formValues.Set("lang", "ar")

	apiEndpoint := fmt.Sprintf("%s/transaction/create", s.cfg.BaseURL)
	httpReq, err := http.NewRequest("POST", apiEndpoint, strings.NewReader(formValues.Encode()))
	if err != nil {
		return "", fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("zaincash api request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	var zResp struct {
		ID  string `json:"id"`
		Msg string `json:"msg"`
		Err string `json:"err"`
	}
	if err := json.Unmarshal(bodyBytes, &zResp); err != nil {
		return "", fmt.Errorf("unmarshal zaincash response (%s): %w", string(bodyBytes), err)
	}

	if zResp.ID == "" {
		errMsg := zResp.Msg
		if errMsg == "" {
			errMsg = zResp.Err
		}
		if errMsg == "" {
			errMsg = string(bodyBytes)
		}
		return "", fmt.Errorf("zaincash transaction failed: %s", errMsg)
	}

	return fmt.Sprintf("%s/transaction/pay?id=%s", s.cfg.BaseURL, zResp.ID), nil
}

// AgentZainCashHandler handles local agent ZainCash endpoints
var AgentZainCashSvc = NewAgentZainCashService()

func AgentGetPricingHandler(c *fiber.Ctx) error {
	price := 1000
	if DB != nil {
		var priceStr string
		err := DB.QueryRow("SELECT value FROM radius_settings WHERE key = 'daily_price_iqd'").Scan(&priceStr)
		if err == nil && priceStr != "" {
			_, _ = fmt.Sscanf(priceStr, "%d", &price)
		}
	}
	return c.JSON(fiber.Map{
		"price_per_day_iqd": price,
		"currency":          "IQD",
	})
}

func AgentSetPricingHandler(c *fiber.Ctx) error {
	var body struct {
		PricePerDayIQD int `json:"price_per_day_iqd"`
	}
	if err := c.BodyParser(&body); err != nil || body.PricePerDayIQD <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid daily price"})
	}
	if DB != nil {
		priceStr := fmt.Sprintf("%d", body.PricePerDayIQD)
		_, err := DB.Exec(`
			INSERT INTO radius_settings (key, value) VALUES ('daily_price_iqd', ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, priceStr)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.JSON(fiber.Map{"success": true, "price_per_day_iqd": body.PricePerDayIQD})
}

func AgentInitiatePaymentHandler(c *fiber.Ctx) error {
	var req struct {
		Days     int    `json:"days"`
		Username string `json:"username"`
	}
	if err := c.BodyParser(&req); err != nil || req.Days <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid days count"})
	}

	pricePerDay := 1000
	if DB != nil {
		var priceStr string
		err := DB.QueryRow("SELECT value FROM radius_settings WHERE key = 'daily_price_iqd'").Scan(&priceStr)
		if err == nil && priceStr != "" {
			_, _ = fmt.Sscanf(priceStr, "%d", &pricePerDay)
		}
	}

	totalAmountIQD := req.Days * pricePerDay
	if totalAmountIQD < 250 {
		totalAmountIQD = 250
	}

	orderID := fmt.Sprintf("agent_ord_%d_%d", time.Now().Unix(), req.Days)

	// Register in agent user transactions as pending
	if req.Username != "" {
		_ = addUserTransaction(req.Username, "debt", float64(totalAmountIQD), fmt.Sprintf("طلب تمديد %d يوم عبر زين كاش (Order: %s)", req.Days, orderID), 1)
	}

	scheme := "https"
	if strings.HasPrefix(c.Protocol(), "http") && !c.Secure() {
		scheme = c.Protocol()
	}
	callbackURL := fmt.Sprintf("%s://%s/api/agent/zaincash/callback", scheme, c.Hostname())

	paymentURL, err := AgentZainCashSvc.CreateTransaction(AgentCreateTxReq{
		Amount:      totalAmountIQD,
		ServiceName: fmt.Sprintf("SASMAN Local Payment (%d Days)", req.Days),
		OrderID:     orderID,
		RedirectURL: callbackURL,
	})
	if err != nil {
		log.Printf("[AgentZainCash] ❌ Payment initiation failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":     true,
		"order_id":    orderID,
		"amount_iqd":  totalAmountIQD,
		"payment_url": paymentURL,
	})
}

func AgentZainCashCallbackHandler(c *fiber.Ctx) error {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		return c.Redirect("/#/license?payment=failed&reason=missing_token")
	}

	claims, err := AgentZainCashSvc.VerifyJWT(tokenStr)
	if err != nil {
		log.Printf("[AgentZainCash] ❌ JWT verification failed: %v", err)
		return c.Redirect("/#/license?payment=failed&reason=invalid_token")
	}

	status, _ := claims["status"].(string)
	orderID, _ := claims["orderId"].(string)
	zTransID, _ := claims["id"].(string)

	if strings.ToLower(status) == "success" {
		log.Printf("[AgentZainCash] 🎉 Payment success for Order [%s] Trans [%s]", orderID, zTransID)
		return c.Redirect(fmt.Sprintf("/#/license?payment=success&order=%s", orderID))
	}

	return c.Redirect("/#/license?payment=failed&reason=rejected")
}
