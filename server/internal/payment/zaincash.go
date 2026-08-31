package payment

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
	"strings"
	"time"
)

type ZainCashConfig struct {
	MerchantID string
	Secret     string
	MSISDN     string
	BaseURL    string // e.g. https://api.zaincash.iq or https://test.zaincash.iq
}

type ZainCashService struct {
	cfg ZainCashConfig
}

func NewZainCashService(cfg ZainCashConfig) *ZainCashService {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.zaincash.iq"
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	return &ZainCashService{cfg: cfg}
}

// GenerateJWT creates an HMAC-SHA256 JWT token for ZainCash
// ZainCash expects: header.payload.signature (base64url, no padding)
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
	Amount      int    // Amount in IQD (minimum 250)
	ServiceName string // e.g. "SASMAN License Renewal"
	OrderID     string // Unique order identifier
	RedirectURL string // Callback URL after payment
}

type CreateTransactionResponse struct {
	TransactionID string `json:"id"`
	PaymentURL    string `json:"payment_url"`
}

// CreateTransaction initiates a ZainCash payment transaction.
// ZainCash API docs: POST /transaction/create
// Form fields: token (JWT), merchantId, lang
// JWT claims: amount, serviceType, msisdn, orderId, redirectUrl, iat, exp
func (s *ZainCashService) CreateTransaction(req CreateTransactionRequest) (*CreateTransactionResponse, error) {
	if req.Amount < 250 {
		req.Amount = 250
	}

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

	apiEndpoint := fmt.Sprintf("%s/transaction/create", s.cfg.BaseURL)
	log.Printf("[ZainCash] ▶ POST %s | merchantId=%s | amount=%d | orderId=%s | redirectUrl=%s",
		apiEndpoint, s.cfg.MerchantID, req.Amount, req.OrderID, req.RedirectURL)

	httpReq, err := http.NewRequest("POST", apiEndpoint, strings.NewReader(formValues.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("zaincash api request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	log.Printf("[ZainCash] ◀ HTTP %d | body: %s", resp.StatusCode, string(bodyBytes))

	// ZainCash may return HTML on error (e.g., Cloudflare block or 5xx)
	if resp.StatusCode != http.StatusOK || (len(bodyBytes) > 0 && bodyBytes[0] == '<') {
		return nil, fmt.Errorf("zaincash returned non-JSON response (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
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
	log.Printf("[ZainCash] ✅ Transaction created: id=%s paymentURL=%s", zResp.ID, paymentURL)

	return &CreateTransactionResponse{
		TransactionID: zResp.ID,
		PaymentURL:    paymentURL,
	}, nil
}
