package payment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAlQasehService_CreatePayment(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/egw/payments/create" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Errorf("missing Authorization header")
		}

		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)

		if payload["amount"] != float64(30000) {
			t.Errorf("expected amount 30000, got %v", payload["amount"])
		}
		if payload["order_id"] != "test_ord_123" {
			t.Errorf("expected order_id test_ord_123, got %v", payload["order_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"payment_id": "qaseh_tx_999",
			"token":      "tok_secret_sample_123",
		})
	}))
	defer mockServer.Close()

	svc := NewAlQasehService(AlQasehConfig{
		ClientID:     "NjE3MDAyMA==@SASMAN",
		ClientSecret: "iJ3qAeqdnMJolduGwmlsGKwTpyGwOnCd",
		BaseURL:      mockServer.URL,
		PayURL:       "https://pay.alqaseh.com/pay",
	})

	res, err := svc.CreatePayment(CreateAlQasehPaymentRequest{
		Amount:      30000,
		OrderID:     "test_ord_123",
		Description: "SASMAN License Renewal (30 Days)",
		RedirectURL: "https://sas-man.com/api/payment/alqaseh/callback",
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if res.PaymentID != "qaseh_tx_999" {
		t.Errorf("expected payment_id qaseh_tx_999, got %s", res.PaymentID)
	}
	if res.Token != "tok_secret_sample_123" {
		t.Errorf("expected token tok_secret_sample_123, got %s", res.Token)
	}
	if res.PaymentURL != "https://pay.alqaseh.com/pay/tok_secret_sample_123" {
		t.Errorf("expected payment_url https://pay.alqaseh.com/pay/tok_secret_sample_123, got %s", res.PaymentURL)
	}
}

func TestAlQasehService_GetPaymentStatus(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/egw/payments/qaseh_tx_999" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"payment_id":     "qaseh_tx_999",
			"order_id":       "test_ord_123",
			"payment_status": "succeeded",
			"amount":         30000,
			"currency":       "IQD",
		})
	}))
	defer mockServer.Close()

	svc := NewAlQasehService(AlQasehConfig{
		ClientID:     "NjE3MDAyMA==@SASMAN",
		ClientSecret: "iJ3qAeqdnMJolduGwmlsGKwTpyGwOnCd",
		BaseURL:      mockServer.URL,
	})

	details, err := svc.GetPaymentStatus("qaseh_tx_999")
	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if details.PaymentStatus != "succeeded" {
		t.Errorf("expected succeeded, got %s", details.PaymentStatus)
	}
	if details.OrderID != "test_ord_123" {
		t.Errorf("expected test_ord_123, got %s", details.OrderID)
	}
}
