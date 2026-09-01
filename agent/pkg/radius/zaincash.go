package radius

import (
	"crypto/hmac"
	"crypto/rand"
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

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

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
		baseURL = "https://pg-api.zaincash.iq"
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

// GenerateJWT creates an HMAC-SHA256 JWT token for ZainCash
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

// VerifyJWT validates a ZainCash response JWT token and returns claims
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

// CreateTransaction calls ZainCash API POST /transaction/create
func (s *AgentZainCashService) CreateTransaction(req AgentCreateTxReq) (string, error) {
	// -------------------------------------------------------------
	// ATTEMPT 1: ZainCash API v2 (OAuth2 + JSON init)
	// -------------------------------------------------------------
	if url, err := s.createTransactionV2(req); err == nil && url != "" {
		log.Printf("[AgentZainCash v2] ✅ Transaction created: paymentURL=%s", url)
		return url, nil
	} else if err != nil {
		log.Printf("[AgentZainCash v2] ℹ️ v2 attempt failed, trying v1 fallback: %v", err)
	}

	// -------------------------------------------------------------
	// ATTEMPT 2: ZainCash API v1 (JWT + Form init)
	// -------------------------------------------------------------
	return s.createTransactionV1(req)
}

func (s *AgentZainCashService) createTransactionV2(req AgentCreateTxReq) (string, error) {
	clientID := s.cfg.MerchantID
	clientSecret := s.cfg.Secret
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing client credentials for v2")
	}

	// Step 1: Get Access Token
	tokenURL := fmt.Sprintf("%s/oauth2/token", s.cfg.BaseURL)
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "payment:read payment:write reverse:write")

	httpReq, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("v2 token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("v2 token call: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("v2 token HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil || tokenRes.AccessToken == "" {
		return "", fmt.Errorf("v2 parse token: %w", err)
	}

	// Step 2: Init Transaction
	initURL := fmt.Sprintf("%s/api/v2/payment-gateway/transaction/init", s.cfg.BaseURL)
	extRef := generateUUID()

	payload := map[string]interface{}{
		"language":            "ar",
		"externalReferenceId": extRef,
		"orderId":             req.OrderID,
		"serviceType":         req.ServiceName,
		"amount": map[string]interface{}{
			"value":    req.Amount,
			"currency": "IQD",
		},
		"redirectUrls": map[string]string{
			"successUrl": req.RedirectURL,
			"failureUrl": req.RedirectURL,
		},
	}
	if s.cfg.MSISDN != "" {
		payload["customer"] = map[string]string{"phone": s.cfg.MSISDN}
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("v2 marshal payload: %w", err)
	}

	pReq, err := http.NewRequest("POST", initURL, strings.NewReader(string(jsonBytes)))
	if err != nil {
		return "", fmt.Errorf("v2 init request: %w", err)
	}
	pReq.Header.Set("Content-Type", "application/json")
	pReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)

	pResp, err := client.Do(pReq)
	if err != nil {
		return "", fmt.Errorf("v2 init call: %w", err)
	}
	pBody, _ := io.ReadAll(pResp.Body)
	pResp.Body.Close()

	if pResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("v2 init HTTP %d: %s", pResp.StatusCode, string(pBody))
	}

	var v2Res struct {
		Status      string `json:"status"`
		RedirectURL string `json:"redirectUrl"`
	}
	if err := json.Unmarshal(pBody, &v2Res); err != nil {
		return "", fmt.Errorf("v2 parse init res: %w", err)
	}

	if v2Res.RedirectURL == "" {
		return "", fmt.Errorf("v2 empty redirectUrl: %s", string(pBody))
	}

	return v2Res.RedirectURL, nil
}

func (s *AgentZainCashService) createTransactionV1(req AgentCreateTxReq) (string, error) {

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

	endpointsToTry := []string{
		fmt.Sprintf("%s/transaction/init", s.cfg.BaseURL),
		fmt.Sprintf("%s/transaction/create", s.cfg.BaseURL),
	}

	var resp *http.Response
	var bodyBytes []byte
	var lastErr error

	for _, apiEndpoint := range endpointsToTry {
		log.Printf("[AgentZainCash] ▶ POST %s | merchantId=%s | amount=%d | orderId=%s | redirectUrl=%s",
			apiEndpoint, s.cfg.MerchantID, req.Amount, req.OrderID, req.RedirectURL)

		httpReq, err := http.NewRequest("POST", apiEndpoint, strings.NewReader(formValues.Encode()))
		if err != nil {
			lastErr = fmt.Errorf("create http request: %w", err)
			continue
		}
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		httpReq.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 20 * time.Second}
		res, err := client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("zaincash api request failed: %w", err)
			continue
		}
		body, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		log.Printf("[AgentZainCash] ◀ HTTP %d | body: %s", res.StatusCode, string(body))
		if res.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("HTTP 404 at %s", apiEndpoint)
			continue
		}

		resp = res
		bodyBytes = body
		lastErr = nil
		break
	}

	if lastErr != nil && len(bodyBytes) == 0 {
		return "", lastErr
	}

	if resp == nil || resp.StatusCode != http.StatusOK || (len(bodyBytes) > 0 && bodyBytes[0] == '<') {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		return "", fmt.Errorf("zaincash returned non-JSON response (HTTP %d): %s", code, string(bodyBytes))
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

	paymentURL := fmt.Sprintf("%s/transaction/pay?id=%s", s.cfg.BaseURL, zResp.ID)
	log.Printf("[AgentZainCash] ✅ Transaction created: id=%s paymentURL=%s", zResp.ID, paymentURL)
	return paymentURL, nil
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
		Gateway  string `json:"gateway"`
	}
	if err := c.BodyParser(&req); err != nil || req.Days <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid days count"})
	}

	gateway := strings.ToLower(strings.TrimSpace(req.Gateway))
	if gateway == "alqaseh" || gateway == "qaseh" {
		return AgentAlQasehInitiatePaymentHandler(c)
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

	orderID := fmt.Sprintf("agent_ord_%d_%s", time.Now().UnixNano(), generateUUID()[:8])

	// Register in agent user transactions as pending
	if req.Username != "" && DB != nil {
		_ = addUserTransaction(req.Username, "debt", float64(totalAmountIQD), fmt.Sprintf("طلب تمديد %d يوم عبر زين كاش (Order: %s)", req.Days, orderID), 1)
	}

	scheme := "https"
	if !c.Secure() {
		scheme = "http"
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
		"gateway":     "zaincash",
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

	if data, ok := claims["data"].(map[string]interface{}); ok {
		if currStatus, ok := data["currentStatus"].(string); ok && currStatus != "" {
			status = currStatus
		}
		if ord, ok := data["orderId"].(string); ok && ord != "" {
			orderID = ord
		}
		if txID, ok := data["transactionId"].(string); ok && txID != "" {
			zTransID = txID
		}
	}

	stLower := strings.ToLower(status)
	if stLower == "success" || stLower == "completed" {
		log.Printf("[AgentZainCash] 🎉 Payment success for Order [%s] Trans [%s]", orderID, zTransID)
		return c.Redirect(fmt.Sprintf("/#/license?payment=success&order=%s", orderID))
	}

	return c.Redirect("/#/license?payment=failed&reason=rejected")
}
