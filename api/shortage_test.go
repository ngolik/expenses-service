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

func TestAddShortageHandler(t *testing.T) {
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
			originalValidator := shortageValidator
			originalRepo := shortageRepo
			shortageValidator = tt.validator
			shortageRepo = tt.repo
			defer func() {
				shortageValidator = originalValidator
				shortageRepo = originalRepo
			}()

			router := setupRouterForTest()
			w := performRequest(router, http.MethodPost, "/expenses/rest/shortage", tt.body)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == 200 {
				var resp ShortageResponse
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

func TestAddShortageHandler_MalformedBody(t *testing.T) {
	originalValidator := shortageValidator
	originalRepo := shortageRepo
	shortageValidator = &fakeValidator{exists: true}
	shortageRepo = &waitingCostFakeRepo{}
	defer func() {
		shortageValidator = originalValidator
		shortageRepo = originalRepo
	}()

	router := setupRouterForTest()
	req := httptest.NewRequest(http.MethodPost, "/expenses/rest/shortage", bytes.NewBufferString("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestGetShortageHandler(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{stored: &model.Expense{DeliveryID: 10, UserID: 5, Amount: 199.99, IsShortage: true}}
		repo.stored.ID = 1

		originalRepo := shortageRepo
		shortageRepo = repo
		defer func() { shortageRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/shortage/1", nil)

		if w.Code != 200 {
			t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
		}

		var resp ShortageResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.DeliveryID != 10 || resp.UserID != 5 || resp.Amount != 199.99 {
			t.Fatalf("unexpected response body: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := shortageRepo
		shortageRepo = repo
		defer func() { shortageRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/shortage/999", nil)

		if w.Code != 404 {
			t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-numeric id", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := shortageRepo
		shortageRepo = repo
		defer func() { shortageRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/shortage/abc", nil)

		if w.Code != 400 {
			t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("other repository error - 500", func(t *testing.T) {
		repo := &waitingCostFakeRepo{findErr: errors.New("db exploded")}
		originalRepo := shortageRepo
		shortageRepo = repo
		defer func() { shortageRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/shortage/1", nil)

		if w.Code != 500 {
			t.Fatalf("status = %d, want 500, body = %s", w.Code, w.Body.String())
		}
	})
}

// TestAC1_HandlerLevel_ShortageDistinguishableFromWaitCostAndDamageWriteOff
// creates a wait-cost record, a damage-writeoff record, and a shortage
// record for the SAME DeliveryID through the HTTP handlers, then reads all
// three back and asserts none of the responses conflate the three record
// kinds (spec AC1, handler level).
func TestAC1_HandlerLevel_ShortageDistinguishableFromWaitCostAndDamageWriteOff(t *testing.T) {
	originalWaitValidator := waitingDeliveryCostValidator
	originalWaitRepo := waitingDeliveryCostRepo
	originalWriteOffValidator := damageWriteOffValidator
	originalWriteOffRepo := damageWriteOffRepo
	originalShortageValidator := shortageValidator
	originalShortageRepo := shortageRepo

	sharedRepo := &waitingCostFakeRepo{}
	validator := &fakeValidator{exists: true}
	waitingDeliveryCostValidator = validator
	waitingDeliveryCostRepo = sharedRepo
	damageWriteOffValidator = validator
	damageWriteOffRepo = sharedRepo
	shortageValidator = validator
	shortageRepo = sharedRepo
	defer func() {
		waitingDeliveryCostValidator = originalWaitValidator
		waitingDeliveryCostRepo = originalWaitRepo
		damageWriteOffValidator = originalWriteOffValidator
		damageWriteOffRepo = originalWriteOffRepo
		shortageValidator = originalShortageValidator
		shortageRepo = originalShortageRepo
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
	if sharedRepo.stored == nil || sharedRepo.stored.IsShortage || sharedRepo.stored.IsDamageWriteOff {
		t.Fatalf("expected the wait-cost record to have IsShortage and IsDamageWriteOff false, got %+v", sharedRepo.stored)
	}

	wWriteOff := performRequest(router, http.MethodPost, "/expenses/rest/damage-writeoff", reqBody{DeliveryID: 500, UserID: 9, Amount: 250.0})
	if wWriteOff.Code != 200 {
		t.Fatalf("damage-writeoff create status = %d, want 200, body = %s", wWriteOff.Code, wWriteOff.Body.String())
	}
	if sharedRepo.stored == nil || sharedRepo.stored.IsShortage || !sharedRepo.stored.IsDamageWriteOff {
		t.Fatalf("expected the damage-writeoff record to have IsShortage false and IsDamageWriteOff true, got %+v", sharedRepo.stored)
	}

	wShortage := performRequest(router, http.MethodPost, "/expenses/rest/shortage", reqBody{DeliveryID: 500, UserID: 11, Amount: 75.0})
	if wShortage.Code != 200 {
		t.Fatalf("shortage create status = %d, want 200, body = %s", wShortage.Code, wShortage.Body.String())
	}
	var shortageResp ShortageResponse
	if err := json.Unmarshal(wShortage.Body.Bytes(), &shortageResp); err != nil {
		t.Fatalf("failed to unmarshal shortage response: %v", err)
	}
	if shortageResp.DeliveryID != 500 || shortageResp.UserID != 11 || shortageResp.Amount != 75.0 {
		t.Fatalf("unexpected shortage response body: %+v", shortageResp)
	}
	// Same DeliveryID (500) as the wait-cost and damage-writeoff records
	// above, but this record must be flagged as a shortage - not conflated
	// with either of the other two amounts for the same delivery (AC1).
	if sharedRepo.stored == nil || !sharedRepo.stored.IsShortage || sharedRepo.stored.IsDamageWriteOff {
		t.Fatalf("expected the shortage record to have IsShortage true and IsDamageWriteOff false, got %+v", sharedRepo.stored)
	}
	if sharedRepo.stored.Amount != 75.0 {
		t.Fatalf("expected the shortage record's amount to be 75.0 (not conflated with the other two records), got %v", sharedRepo.stored.Amount)
	}
}
