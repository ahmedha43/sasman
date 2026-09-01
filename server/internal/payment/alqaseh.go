package payment

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type AlQasehConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string // e.g. https://api.alqaseh.com/v1 or https://api-test.alqaseh.com/v1
	PayURL       string // e.g. https://pay.alqaseh.com/pay or https://pay-test.alqaseh.com/pay
}

type AlQasehService struct {
	cfg AlQasehConfig
}

func NewAlQasehService(cfg AlQasehConfig) *AlQasehService {
	if cfg.ClientID == "" {
		cfg.ClientID = "NjE3MDAyMA==@SASMAN"
	}
	if cfg.ClientSecret == "" {
		cfg.ClientSecret = "iJ3qAeqdnMJolduGwmlsGKwTpyGwOnCd"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.alqaseh.com/v1"
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")

	if cfg.PayURL == "" {
		cfg.PayURL = "https://pay.alqaseh.com/pay"
	}
	cfg.PayURL = strings.TrimSuffix(cfg.PayURL, "/")

	return &AlQasehService{cfg: cfg}
}

func (s *AlQasehService) getBasicAuthHeader() string {
	auth := fmt.Sprintf("%s:%s", s.cfg.ClientID, s.cfg.ClientSecret)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

type CreateAlQasehPaymentRequest struct {
	Amount          int    // Amount in IQD
	Currency        string // "IQD"
	Description     string // Description of order
	OrderID         string // Internal order identifier
	RedirectURL     string // User return callback URL
	WebhookURL      string // Server-to-server webhook URL
	TransactionType string // "Retail", "Authorization", etc.
}

type CreateAlQasehPaymentResponse struct {
	PaymentID  string `json:"payment_id"`
	Token      string `json:"token"`
	PaymentURL string `json:"payment_url"`
}

type AlQasehPaymentDetails struct {
	ID            int                    `json:"id,omitempty"`
	PaymentID     string                 `json:"payment_id"`
	OrderID       string                 `json:"order_id"`
	PaymentStatus string                 `json:"payment_status"` // "prepared", "succeeded", "failed", "revoked", "expired", etc.
	Amount        float64                `json:"amount"`
	Currency      string                 `json:"currency"`
	Description   string                 `json:"description"`
	ApprovalCode  string                 `json:"approval_code,omitempty"`
	RRN           string                 `json:"rrn,omitempty"`
	RedirectURL   string                 `json:"redirect_url,omitempty"`
	WebhookURL    string                 `json:"webhook_url,omitempty"`
	Token         string                 `json:"token,omitempty"`
	CustomData    map[string]interface{} `json:"custom_data,omitempty"`
	CreatedAt     string                 `json:"created_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at,omitempty"`
}

// CreatePayment initiates a new payment context with Al-Qaseh Payment Gateway
func (s *AlQasehService) CreatePayment(req CreateAlQasehPaymentRequest) (*CreateAlQasehPaymentResponse, error) {
	if req.Amount < 250 {
		req.Amount = 250
	}
	if req.Currency == "" {
		req.Currency = "IQD"
	}
	if req.TransactionType == "" {
		req.TransactionType = "Retail"
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
		"transaction_type": req.TransactionType,
		"redirect_url":     req.RedirectURL,
	}
	if req.WebhookURL != "" {
		payload["webhook_url"] = req.WebhookURL
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal alqaseh request: %w", err)
	}

	log.Printf("[AlQaseh] ▶ POST %s | orderId=%s | amount=%d %s | redirectUrl=%s",
		createURL, req.OrderID, req.Amount, req.Currency, req.RedirectURL)

	httpReq, err := http.NewRequest("POST", createURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", s.getBasicAuthHeader())

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("alqaseh api call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read alqaseh response: %w", err)
	}

	log.Printf("[AlQaseh] ◀ HTTP %d | body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("alqaseh returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var createRes struct {
		PaymentID string `json:"payment_id"`
		Token     string `json:"token"`
		Err       string `json:"err"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(body, &createRes); err != nil {
		return nil, fmt.Errorf("unmarshal alqaseh response: %w", err)
	}

	if createRes.Token == "" && createRes.PaymentID == "" {
		return nil, fmt.Errorf("alqaseh response missing token/payment_id: %s", string(body))
	}

	// Build hosted checkout redirect URL
	paymentURL := fmt.Sprintf("%s/%s", s.cfg.PayURL, createRes.Token)
	log.Printf("[AlQaseh] ✅ Payment context created: payment_id=%s token=%s payURL=%s",
		createRes.PaymentID, createRes.Token, paymentURL)

	return &CreateAlQasehPaymentResponse{
		PaymentID:  createRes.PaymentID,
		Token:      createRes.Token,
		PaymentURL: paymentURL,
	}, nil
}

// GetPaymentStatus queries Al-Qaseh API by PaymentID to verify transaction status directly
func (s *AlQasehService) GetPaymentStatus(paymentID string) (*AlQasehPaymentDetails, error) {
	if paymentID == "" {
		return nil, fmt.Errorf("empty payment_id")
	}

	infoURL := fmt.Sprintf("%s/egw/payments/%s", s.cfg.BaseURL, paymentID)
	httpReq, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", s.getBasicAuthHeader())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("alqaseh get status call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[AlQaseh] 🔍 GET /egw/payments/%s ◀ HTTP %d | body: %s", paymentID, resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alqaseh HTTP %d: %s", resp.StatusCode, string(body))
	}

	var details AlQasehPaymentDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, fmt.Errorf("parse alqaseh payment details: %w", err)
	}

	return &details, nil
}

// GetPaymentInfoByToken queries essential payment details by token
func (s *AlQasehService) GetPaymentInfoByToken(token string) (*AlQasehPaymentDetails, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	infoURL := fmt.Sprintf("%s/egw/payments/info/%s", s.cfg.BaseURL, token)
	httpReq, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", s.getBasicAuthHeader())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("alqaseh get info by token call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[AlQaseh] 🔍 GET /egw/payments/info/%s ◀ HTTP %d | body: %s", token, resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alqaseh token info HTTP %d: %s", resp.StatusCode, string(body))
	}

	var details AlQasehPaymentDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, fmt.Errorf("parse alqaseh token details: %w", err)
	}
	details.Token = token

	return &details, nil
}
