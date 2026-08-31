package payment

import (
	"bytes"
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
	"strings"
	"time"
)

type ZainCashConfig struct {
	MerchantID   string
	Secret       string
	ClientID     string
	ClientSecret string
	MSISDN       string
	BaseURL      string // e.g. https://pg-api-uat.zaincash.iq or https://pg-api.zaincash.iq
}

type ZainCashService struct {
	cfg ZainCashConfig
}

func NewZainCashService(cfg ZainCashConfig) *ZainCashService {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://pg-api.zaincash.iq"
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	if cfg.ClientID == "" {
		cfg.ClientID = cfg.MerchantID
	}
	if cfg.ClientSecret == "" {
		cfg.ClientSecret = cfg.Secret
	}
	return &ZainCashService{cfg: cfg}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// GenerateJWT creates an HMAC-SHA256 JWT token for ZainCash (v1)
func (s *ZainCashService) GenerateJWT(claims map[string]interface{}) (string, error) {
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
func (s *ZainCashService) VerifyJWT(tokenStr string) (map[string]interface{}, error) {
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

type CreateTransactionRequest struct {
	Amount        int    // Amount in IQD (minimum 250)
	ServiceName   string // e.g. "SASMAN License Renewal"
	OrderID       string // Unique order identifier
	RedirectURL   string // Callback URL after payment
	CustomerPhone string // Optional: saved wallet number for returning customers
}

type CreateTransactionResponse struct {
	TransactionID string `json:"id"`
	PaymentURL    string `json:"payment_url"`
}

// CreateTransaction initiates a ZainCash payment transaction.
// It attempts ZainCash API v2 (OAuth2 + /api/v2/payment-gateway/transaction/init) first,
// and falls back to ZainCash v1 JWT flow if v2 is unavailable.
func (s *ZainCashService) CreateTransaction(req CreateTransactionRequest) (*CreateTransactionResponse, error) {
	if req.Amount < 250 {
		req.Amount = 250
	}

	// -------------------------------------------------------------
	// ATTEMPT 1: ZainCash API v2 (OAuth2 + JSON init)
	// -------------------------------------------------------------
	if res, err := s.createTransactionV2(req); err == nil && res != nil && res.PaymentURL != "" {
		log.Printf("[ZainCash v2] ✅ Transaction created: id=%s paymentURL=%s", res.TransactionID, res.PaymentURL)
		return res, nil
	} else if err != nil {
		log.Printf("[ZainCash v2] ℹ️ v2 attempt failed, trying v1 fallback: %v", err)
	}

	// -------------------------------------------------------------
	// ATTEMPT 2: ZainCash API v1 (JWT + Form init)
	// -------------------------------------------------------------
	return s.createTransactionV1(req)
}

func (s *ZainCashService) createTransactionV2(req CreateTransactionRequest) (*CreateTransactionResponse, error) {
	clientID := s.cfg.ClientID
	clientSecret := s.cfg.ClientSecret
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("missing client credentials for v2")
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
		return nil, fmt.Errorf("v2 token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("v2 token call: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("v2 token HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenRes); err != nil || tokenRes.AccessToken == "" {
		return nil, fmt.Errorf("v2 parse token: %w", err)
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
	// Only pass customer.phone if we have a saved wallet number for this customer.
	// Per ZainCash docs: omit customer.phone on first payment → ZainCash prompts user to enter their wallet number.
	// Pass it on subsequent payments → ZainCash skips the phone entry step.
	if req.CustomerPhone != "" {
		payload["customer"] = map[string]string{"phone": req.CustomerPhone}
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("v2 marshal payload: %w", err)
	}

	pReq, err := http.NewRequest("POST", initURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("v2 init request: %w", err)
	}
	pReq.Header.Set("Content-Type", "application/json")
	pReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)

	pResp, err := client.Do(pReq)
	if err != nil {
		return nil, fmt.Errorf("v2 init call: %w", err)
	}
	pBody, _ := io.ReadAll(pResp.Body)
	pResp.Body.Close()

	if pResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("v2 init HTTP %d: %s", pResp.StatusCode, string(pBody))
	}

	var v2Res struct {
		Status             string `json:"status"`
		RedirectURL        string `json:"redirectUrl"`
		TransactionDetails struct {
			TransactionID string `json:"transactionId"`
		} `json:"transactionDetails"`
	}
	if err := json.Unmarshal(pBody, &v2Res); err != nil {
		return nil, fmt.Errorf("v2 parse init res: %w", err)
	}

	if v2Res.RedirectURL == "" {
		return nil, fmt.Errorf("v2 empty redirectUrl: %s", string(pBody))
	}

	txID := v2Res.TransactionDetails.TransactionID
	if txID == "" {
		txID = extRef
	}

	return &CreateTransactionResponse{
		TransactionID: txID,
		PaymentURL:    v2Res.RedirectURL,
	}, nil
}

func (s *ZainCashService) createTransactionV1(req CreateTransactionRequest) (*CreateTransactionResponse, error) {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"amount":      req.Amount,
		"serviceType": req.ServiceName,
		"msisdn":      s.cfg.MSISDN,
		"orderId":     req.OrderID,
		"redirectUrl": req.RedirectURL,
		"iat":         now,
		"exp":         now + 4*3600, // 4 hours
	}

	token, err := s.GenerateJWT(claims)
	if err != nil {
		return nil, fmt.Errorf("generate jwt: %w", err)
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
		log.Printf("[ZainCash v1] ▶ POST %s | merchantId=%s | amount=%d | orderId=%s | redirectUrl=%s",
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

		log.Printf("[ZainCash v1] ◀ HTTP %d | body: %s", res.StatusCode, string(body))
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
		return nil, lastErr
	}

	if resp == nil || resp.StatusCode != http.StatusOK || (len(bodyBytes) > 0 && bodyBytes[0] == '<') {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		return nil, fmt.Errorf("zaincash returned non-JSON response (HTTP %d): %s", code, string(bodyBytes))
	}

	var zResp struct {
		ID   string `json:"id"`
		Err  string `json:"err"`
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(bodyBytes, &zResp); err != nil {
		return nil, fmt.Errorf("unmarshal zaincash response (%s): %w", string(bodyBytes), err)
	}

	if zResp.ID == "" {
		errMsg := zResp.Msg
		if errMsg == "" {
			errMsg = zResp.Err
		}
		if errMsg == "" {
			errMsg = string(bodyBytes)
		}
		return nil, fmt.Errorf("zaincash transaction failed: %s", errMsg)
	}

	paymentURL := fmt.Sprintf("%s/transaction/pay?id=%s", s.cfg.BaseURL, zResp.ID)
	log.Printf("[ZainCash v1] ✅ Transaction created: id=%s paymentURL=%s", zResp.ID, paymentURL)

	return &CreateTransactionResponse{
		TransactionID: zResp.ID,
		PaymentURL:    paymentURL,
	}, nil
}
