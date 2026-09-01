package radius

import (
	"bytes"
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

type AgentAlQasehConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	PayURL       string
}

type AgentAlQasehService struct {
	cfg AgentAlQasehConfig
}

func NewAgentAlQasehService() *AgentAlQasehService {
	clientID := os.Getenv("ALQASEH_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("ALQASEH_API_CLIENT")
	}
	if clientID == "" {
		clientID = "NjE3MDAyMA==@SASMAN"
	}

	clientSecret := os.Getenv("ALQASEH_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("ALQASEH_API_SECRET")
	}
	if clientSecret == "" {
		clientSecret = "iJ3qAeqdnMJolduGwmlsGKwTpyGwOnCd"
	}

	baseURL := os.Getenv("ALQASEH_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.alqaseh.com/v1"
	}

	payURL := os.Getenv("ALQASEH_PAY_URL")
	if payURL == "" {
		payURL = "https://pay.alqaseh.com/pay"
	}

	return &AgentAlQasehService{
		cfg: AgentAlQasehConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			BaseURL:      strings.TrimSuffix(baseURL, "/"),
			PayURL:       strings.TrimSuffix(payURL, "/"),
		},
	}
}

func (s *AgentAlQasehService) getBasicAuthHeader() string {
	auth := fmt.Sprintf("%s:%s", s.cfg.ClientID, s.cfg.ClientSecret)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

type AgentCreateAlQasehReq struct {
	Amount      int
	Currency    string
	Description string
	OrderID     string
	RedirectURL string
	WebhookURL  string
}

type AgentAlQasehDetails struct {
	PaymentID     string  `json:"payment_id"`
	OrderID       string  `json:"order_id"`
	PaymentStatus string  `json:"payment_status"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Token         string  `json:"token,omitempty"`
}

// CreatePayment initiates a payment on Al-Qaseh and returns the hosted payment checkout URL
func (s *AgentAlQasehService) CreatePayment(req AgentCreateAlQasehReq) (string, error) {
	if req.Amount < 250 {
		req.Amount = 250
	}
	if req.Currency == "" {
		req.Currency = "IQD"
	}

	orderID := strings.TrimSpace(req.OrderID)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	createURL := fmt.Sprintf("%s/egw/payments/create", s.cfg.BaseURL)

	payload := map[string]interface{}{
		"amount":           req.Amount,
		"currency":         req.Currency,
		"description":      req.Description,
		"order_id":         orderID,
		"transaction_type": "Retail",
		"redirect_url":     req.RedirectURL,
	}
	if req.WebhookURL != "" {
		payload["webhook_url"] = req.WebhookURL
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	log.Printf("[AgentAlQaseh] ▶ POST %s | orderId=%s | amount=%d %s | redirectUrl=%s",
		createURL, req.OrderID, req.Amount, req.Currency, req.RedirectURL)

	httpReq, err := http.NewRequest("POST", createURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", s.getBasicAuthHeader())

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	log.Printf("[AgentAlQaseh] ◀ HTTP %d | body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("alqaseh returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var createRes struct {
		PaymentID string `json:"payment_id"`
		Token     string `json:"token"`
	}
	if err := json.Unmarshal(body, &createRes); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if createRes.Token == "" {
		return "", fmt.Errorf("missing token in response: %s", string(body))
	}

	paymentURL := fmt.Sprintf("%s/%s", s.cfg.PayURL, createRes.Token)
	log.Printf("[AgentAlQaseh] ✅ Transaction created: payment_id=%s token=%s payURL=%s",
		createRes.PaymentID, createRes.Token, paymentURL)

	return paymentURL, nil
}

// GetPaymentStatus queries payment context status from Al-Qaseh
func (s *AgentAlQasehService) GetPaymentStatus(paymentID string) (*AgentAlQasehDetails, error) {
	if paymentID == "" {
		return nil, fmt.Errorf("empty payment_id")
	}

	infoURL := fmt.Sprintf("%s/egw/payments/%s", s.cfg.BaseURL, paymentID)
	httpReq, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", s.getBasicAuthHeader())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alqaseh HTTP %d: %s", resp.StatusCode, string(body))
	}

	var details AgentAlQasehDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, err
	}

	return &details, nil
}

// GetPaymentInfoByToken queries payment status by token
func (s *AgentAlQasehService) GetPaymentInfoByToken(token string) (*AgentAlQasehDetails, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	infoURL := fmt.Sprintf("%s/egw/payments/info/%s", s.cfg.BaseURL, token)
	httpReq, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", s.getBasicAuthHeader())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alqaseh token HTTP %d: %s", resp.StatusCode, string(body))
	}

	var details AgentAlQasehDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, err
	}
	details.Token = token

	return &details, nil
}

var AgentAlQasehSvc = NewAgentAlQasehService()

// AgentAlQasehInitiatePaymentHandler initiates an Al-Qaseh payment in Agent mode
func AgentAlQasehInitiatePaymentHandler(c *fiber.Ctx) error {
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

	orderID := fmt.Sprintf("ag_ord_%d_%s", time.Now().Unix(), generateUUID()[:8])
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	if req.Username != "" && DB != nil {
		_ = addUserTransaction(req.Username, "debt", float64(totalAmountIQD), fmt.Sprintf("طلب تمديد %d يوم عبر القاصة (Order: %s)", req.Days, orderID), 1)
	}

	scheme := "https"
	if !c.Secure() {
		scheme = "http"
	}
	callbackURL := fmt.Sprintf("%s://%s/api/agent/alqaseh/callback", scheme, c.Hostname())

	paymentURL, err := AgentAlQasehSvc.CreatePayment(AgentCreateAlQasehReq{
		Amount:      totalAmountIQD,
		Currency:    "IQD",
		Description: fmt.Sprintf("SASMAN Local Payment (%d Days)", req.Days),
		OrderID:     orderID,
		RedirectURL: callbackURL,
	})
	if err != nil {
		log.Printf("[AgentAlQaseh] ❌ Payment initiation failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":     true,
		"order_id":    orderID,
		"amount_iqd":  totalAmountIQD,
		"gateway":     "alqaseh",
		"payment_url": paymentURL,
	})
}

// AgentAlQasehCallbackHandler handles Al-Qaseh payment callback in Agent mode
func AgentAlQasehCallbackHandler(c *fiber.Ctx) error {
	paymentID := c.Query("payment_id")
	if paymentID == "" {
		paymentID = c.Query("id")
	}
	tokenStr := c.Query("token")
	orderID := c.Query("order_id")

	log.Printf("[AgentAlQaseh] 💳 Callback received: payment_id=%s token=%s order_id=%s", paymentID, tokenStr, orderID)

	var details *AgentAlQasehDetails
	var err error

	if paymentID != "" {
		details, err = AgentAlQasehSvc.GetPaymentStatus(paymentID)
	} else if tokenStr != "" {
		details, err = AgentAlQasehSvc.GetPaymentInfoByToken(tokenStr)
	}

	if err != nil || details == nil {
		log.Printf("[AgentAlQaseh] ❌ Payment status verification failed: %v", err)
		return c.Redirect("/#/license?payment=failed&gateway=alqaseh&reason=verification_failed")
	}

	if orderID == "" {
		orderID = details.OrderID
	}

	stLower := strings.ToLower(details.PaymentStatus)
	if stLower == "succeeded" || stLower == "success" || stLower == "completed" {
		log.Printf("[AgentAlQaseh] 🎉 Payment SUCCESS for Order [%s] Trans [%s]", orderID, details.PaymentID)
		return c.Redirect(fmt.Sprintf("/#/license?payment=success&gateway=alqaseh&order=%s", orderID))
	}

	return c.Redirect(fmt.Sprintf("/#/license?payment=failed&gateway=alqaseh&reason=%s", url.QueryEscape(details.PaymentStatus)))
}
