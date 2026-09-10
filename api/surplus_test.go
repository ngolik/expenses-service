package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ngolik/expense-service/model"
)

func TestAddSurplusHandler(t *testing.T) {
	type reqBody struct {
		DeliveryID int     `json:"DeliveryID"`
		UserID     int     `json:"UserID"`
		Amount     float64 `json:"Amount"`
	}

	tests := []struct {
		name       string
		body       reqBody
		validator  *fakeValidator
		repo       *waitingCostFakeRepo
		wantStatus int
	}{
		{
			name:       "all fields present, user exists - 200",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 199.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 200,
		},
		{
			name:       "missing delivery id - 400",
			body:       reqBody{UserID: 5, Amount: 199.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "missing user id - 400",
			body:       reqBody{DeliveryID: 10, Amount: 199.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "missing amount - 400",
			body:       reqBody{DeliveryID: 10, UserID: 5},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "unknown user - 400",
			body:       reqBody{DeliveryID: 10, UserID: 999, Amount: 199.99},
			validator:  &fakeValidator{exists: false},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "auth-service call fails - 502",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 199.99},
			validator:  &fakeValidator{err: errors.New("connection refused")},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 502,
		},
		{
			name:       "db create fails - 500",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 199.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{createErr: errors.New("db down")},
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalValidator := surplusValidator
			originalRepo := surplusRepo
			surplusValidator = tt.validator
			surplusRepo = tt.repo
			defer func() {
				surplusValidator = originalValidator
				surplusRepo = originalRepo
			}()

			router := setupRouterForTest()
			w := performRequest(router, http.MethodPost, "/expenses/rest/surplus", tt.body)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == 200 {
				var resp SurplusResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal success response: %v", err)
				}
				if resp.ID == 0 {
					t.Fatalf("expected a non-zero id in the create response, got %+v", resp)
				}
				if resp.DeliveryID != tt.body.DeliveryID || resp.UserID != tt.body.UserID || resp.Amount != tt.body.Amount {
					t.Fatalf("unexpected response body: %+v", resp)
				}
			}
		})
	}
}

func TestAddSurplusHandler_MalformedBody(t *testing.T) {
	originalValidator := surplusValidator
	originalRepo := surplusRepo
	surplusValidator = &fakeValidator{exists: true}
	surplusRepo = &waitingCostFakeRepo{}
	defer func() {
		surplusValidator = originalValidator
		surplusRepo = originalRepo
	}()

	router := setupRouterForTest()
	req := httptest.NewRequest(http.MethodPost, "/expenses/rest/surplus", bytes.NewBufferString("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestGetSurplusHandler(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{stored: &model.Expense{DeliveryID: 10, UserID: 5, Amount: 199.99, IsSurplus: true}}
		repo.stored.ID = 1

		originalRepo := surplusRepo
		surplusRepo = repo
		defer func() { surplusRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/surplus/1", nil)

		if w.Code != 200 {
			t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
		}

		var resp SurplusResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.DeliveryID != 10 || resp.UserID != 5 || resp.Amount != 199.99 {
			t.Fatalf("unexpected response body: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := surplusRepo
		surplusRepo = repo
		defer func() { surplusRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/surplus/999", nil)

		if w.Code != 404 {
			t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-numeric id", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := surplusRepo
		surplusRepo = repo
		defer func() { surplusRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/surplus/abc", nil)

		if w.Code != 400 {
			t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("other repository error - 500", func(t *testing.T) {
		repo := &waitingCostFakeRepo{findErr: errors.New("db exploded")}
		originalRepo := surplusRepo
		surplusRepo = repo
		defer func() { surplusRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/surplus/1", nil)

		if w.Code != 500 {
			t.Fatalf("status = %d, want 500, body = %s", w.Code, w.Body.String())
		}
	})
}

// TestAC1_HandlerLevel_SurplusDistinguishableFromWaitCostDamageWriteOffAndShortage
// creates a wait-cost record, a damage-writeoff record, a shortage record,
// and a surplus record for the SAME DeliveryID through the HTTP handlers,
// then reads all four back and asserts none of the responses conflate the
// four record kinds (spec AC1, handler level).
func TestAC1_HandlerLevel_SurplusDistinguishableFromWaitCostDamageWriteOffAndShortage(t *testing.T) {
	originalWaitValidator := waitingDeliveryCostValidator
	originalWaitRepo := waitingDeliveryCostRepo
	originalWriteOffValidator := damageWriteOffValidator
	originalWriteOffRepo := damageWriteOffRepo
	originalShortageValidator := shortageValidator
	originalShortageRepo := shortageRepo
	originalSurplusValidator := surplusValidator
	originalSurplusRepo := surplusRepo

	sharedRepo := &waitingCostFakeRepo{}
	validator := &fakeValidator{exists: true}
	waitingDeliveryCostValidator = validator
	waitingDeliveryCostRepo = sharedRepo
	damageWriteOffValidator = validator
	damageWriteOffRepo = sharedRepo
	shortageValidator = validator
	shortageRepo = sharedRepo
	surplusValidator = validator
	surplusRepo = sharedRepo
	defer func() {
		waitingDeliveryCostValidator = originalWaitValidator
		waitingDeliveryCostRepo = originalWaitRepo
		damageWriteOffValidator = originalWriteOffValidator
		damageWriteOffRepo = originalWriteOffRepo
		shortageValidator = originalShortageValidator
		shortageRepo = originalShortageRepo
		surplusValidator = originalSurplusValidator
		surplusRepo = originalSurplusRepo
	}()

	router := setupRouterForTest()

	type reqBody struct {
		DeliveryID int     `json:"DeliveryID"`
		UserID     int     `json:"UserID"`
		Amount     float64 `json:"Amount"`
	}

	wWait := performRequest(router, http.MethodPost, "/expenses/rest/waiting-cost", reqBody{DeliveryID: 500, UserID: 7, Amount: 15.0})
	if wWait.Code != 200 {
		t.Fatalf("wait-cost create status = %d, want 200, body = %s", wWait.Code, wWait.Body.String())
	}
	if sharedRepo.stored == nil || sharedRepo.stored.IsShortage || sharedRepo.stored.IsDamageWriteOff || sharedRepo.stored.IsSurplus {
		t.Fatalf("expected the wait-cost record to have IsShortage/IsDamageWriteOff/IsSurplus false, got %+v", sharedRepo.stored)
	}

	wWriteOff := performRequest(router, http.MethodPost, "/expenses/rest/damage-writeoff", reqBody{DeliveryID: 500, UserID: 9, Amount: 250.0})
	if wWriteOff.Code != 200 {
		t.Fatalf("damage-writeoff create status = %d, want 200, body = %s", wWriteOff.Code, wWriteOff.Body.String())
	}
	if sharedRepo.stored == nil || sharedRepo.stored.IsShortage || !sharedRepo.stored.IsDamageWriteOff || sharedRepo.stored.IsSurplus {
		t.Fatalf("expected the damage-writeoff record to have IsShortage/IsSurplus false and IsDamageWriteOff true, got %+v", sharedRepo.stored)
	}

	wShortage := performRequest(router, http.MethodPost, "/expenses/rest/shortage", reqBody{DeliveryID: 500, UserID: 11, Amount: 75.0})
	if wShortage.Code != 200 {
		t.Fatalf("shortage create status = %d, want 200, body = %s", wShortage.Code, wShortage.Body.String())
	}
	if sharedRepo.stored == nil || !sharedRepo.stored.IsShortage || sharedRepo.stored.IsDamageWriteOff || sharedRepo.stored.IsSurplus {
		t.Fatalf("expected the shortage record to have IsShortage true and IsDamageWriteOff/IsSurplus false, got %+v", sharedRepo.stored)
	}

	wSurplus := performRequest(router, http.MethodPost, "/expenses/rest/surplus", reqBody{DeliveryID: 500, UserID: 13, Amount: 30.0})
	if wSurplus.Code != 200 {
		t.Fatalf("surplus create status = %d, want 200, body = %s", wSurplus.Code, wSurplus.Body.String())
	}
	var surplusResp SurplusResponse
	if err := json.Unmarshal(wSurplus.Body.Bytes(), &surplusResp); err != nil {
		t.Fatalf("failed to unmarshal surplus response: %v", err)
	}
	if surplusResp.DeliveryID != 500 || surplusResp.UserID != 13 || surplusResp.Amount != 30.0 {
		t.Fatalf("unexpected surplus response body: %+v", surplusResp)
	}
	// Same DeliveryID (500) as the wait-cost, damage-writeoff, and shortage
	// records above, but this record must be flagged as a surplus - not
	// conflated with any of the other three amounts for the same delivery
	// (AC1).
	if sharedRepo.stored == nil || !sharedRepo.stored.IsSurplus || sharedRepo.stored.IsShortage || sharedRepo.stored.IsDamageWriteOff {
		t.Fatalf("expected the surplus record to have IsSurplus true and IsShortage/IsDamageWriteOff false, got %+v", sharedRepo.stored)
	}
	if sharedRepo.stored.Amount != 30.0 {
		t.Fatalf("expected the surplus record's amount to be 30.0 (not conflated with the other three records), got %v", sharedRepo.stored.Amount)
	}
}
