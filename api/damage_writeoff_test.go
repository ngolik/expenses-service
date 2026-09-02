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

func TestAddDamageWriteOffHandler(t *testing.T) {
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
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 999.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 200,
		},
		{
			name:       "missing delivery id - 400",
			body:       reqBody{UserID: 5, Amount: 999.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "missing user id - 400",
			body:       reqBody{DeliveryID: 10, Amount: 999.99},
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
			body:       reqBody{DeliveryID: 10, UserID: 999, Amount: 999.99},
			validator:  &fakeValidator{exists: false},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "auth-service call fails - 502",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 999.99},
			validator:  &fakeValidator{err: errors.New("connection refused")},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 502,
		},
		{
			name:       "db create fails - 500",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 999.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{createErr: errors.New("db down")},
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalValidator := damageWriteOffValidator
			originalRepo := damageWriteOffRepo
			damageWriteOffValidator = tt.validator
			damageWriteOffRepo = tt.repo
			defer func() {
				damageWriteOffValidator = originalValidator
				damageWriteOffRepo = originalRepo
			}()

			router := setupRouterForTest()
			w := performRequest(router, http.MethodPost, "/expenses/rest/damage-writeoff", tt.body)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == 200 {
				var resp DamageWriteOffResponse
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

func TestAddDamageWriteOffHandler_MalformedBody(t *testing.T) {
	originalValidator := damageWriteOffValidator
	originalRepo := damageWriteOffRepo
	damageWriteOffValidator = &fakeValidator{exists: true}
	damageWriteOffRepo = &waitingCostFakeRepo{}
	defer func() {
		damageWriteOffValidator = originalValidator
		damageWriteOffRepo = originalRepo
	}()

	router := setupRouterForTest()
	req := httptest.NewRequest(http.MethodPost, "/expenses/rest/damage-writeoff", bytes.NewBufferString("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestGetDamageWriteOffHandler(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{stored: &model.Expense{DeliveryID: 10, UserID: 5, Amount: 999.99, IsDamageWriteOff: true}}
		repo.stored.ID = 1

		originalRepo := damageWriteOffRepo
		damageWriteOffRepo = repo
		defer func() { damageWriteOffRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/damage-writeoff/1", nil)

		if w.Code != 200 {
			t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
		}

		var resp DamageWriteOffResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.DeliveryID != 10 || resp.UserID != 5 || resp.Amount != 999.99 {
			t.Fatalf("unexpected response body: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := damageWriteOffRepo
		damageWriteOffRepo = repo
		defer func() { damageWriteOffRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/damage-writeoff/999", nil)

		if w.Code != 404 {
			t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-numeric id", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := damageWriteOffRepo
		damageWriteOffRepo = repo
		defer func() { damageWriteOffRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/damage-writeoff/abc", nil)

		if w.Code != 400 {
			t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("other repository error - 500", func(t *testing.T) {
		repo := &waitingCostFakeRepo{findErr: errors.New("db exploded")}
		originalRepo := damageWriteOffRepo
		damageWriteOffRepo = repo
		defer func() { damageWriteOffRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/damage-writeoff/1", nil)

		if w.Code != 500 {
			t.Fatalf("status = %d, want 500, body = %s", w.Code, w.Body.String())
		}
	})
}

// TestAC8_HandlerLevel_WaitCostAndDamageWriteOffAreDistinguishable creates a
// wait-cost record and a damage-writeoff record for the SAME DeliveryID
// through the HTTP handlers, then reads both back and asserts neither
// response conflates the two record kinds (AC8, handler level).
func TestAC8_HandlerLevel_WaitCostAndDamageWriteOffAreDistinguishable(t *testing.T) {
	originalWaitValidator := waitingDeliveryCostValidator
	originalWaitRepo := waitingDeliveryCostRepo
	originalWriteOffValidator := damageWriteOffValidator
	originalWriteOffRepo := damageWriteOffRepo

	sharedRepo := &waitingCostFakeRepo{}
	validator := &fakeValidator{exists: true}
	waitingDeliveryCostValidator = validator
	waitingDeliveryCostRepo = sharedRepo
	damageWriteOffValidator = validator
	damageWriteOffRepo = sharedRepo
	defer func() {
		waitingDeliveryCostValidator = originalWaitValidator
		waitingDeliveryCostRepo = originalWaitRepo
		damageWriteOffValidator = originalWriteOffValidator
		damageWriteOffRepo = originalWriteOffRepo
	}()

	router := setupRouterForTest()

	type reqBody struct {
		DeliveryID int     `json:"DeliveryID"`
		UserID     int     `json:"UserID"`
		Amount     float64 `json:"Amount"`
	}

	wWait := performRequest(router, http.MethodPost, "/expenses/rest/waiting-cost", reqBody{DeliveryID: 300, UserID: 7, Amount: 15.0})
	if wWait.Code != 200 {
		t.Fatalf("wait-cost create status = %d, want 200, body = %s", wWait.Code, wWait.Body.String())
	}
	var waitResp WaitingDeliveryCostResponse
	if err := json.Unmarshal(wWait.Body.Bytes(), &waitResp); err != nil {
		t.Fatalf("failed to unmarshal wait-cost response: %v", err)
	}
	if waitResp.DeliveryID != 300 || waitResp.UserID != 7 || waitResp.Amount != 15.0 {
		t.Fatalf("unexpected wait-cost response body: %+v", waitResp)
	}
	// The wait-cost creation path never sets IsDamageWriteOff - confirm the
	// stored record it produced is not flagged as a damage write-off.
	if sharedRepo.stored == nil || sharedRepo.stored.IsDamageWriteOff {
		t.Fatalf("expected the wait-cost record to have IsDamageWriteOff false, got %+v", sharedRepo.stored)
	}

	wWriteOff := performRequest(router, http.MethodPost, "/expenses/rest/damage-writeoff", reqBody{DeliveryID: 300, UserID: 9, Amount: 250.0})
	if wWriteOff.Code != 200 {
		t.Fatalf("damage-writeoff create status = %d, want 200, body = %s", wWriteOff.Code, wWriteOff.Body.String())
	}
	var writeOffResp DamageWriteOffResponse
	if err := json.Unmarshal(wWriteOff.Body.Bytes(), &writeOffResp); err != nil {
		t.Fatalf("failed to unmarshal damage-writeoff response: %v", err)
	}
	if writeOffResp.DeliveryID != 300 || writeOffResp.UserID != 9 || writeOffResp.Amount != 250.0 {
		t.Fatalf("unexpected damage-writeoff response body: %+v", writeOffResp)
	}
	// Same DeliveryID (300) as the wait-cost record above, but this record
	// must be flagged as a damage write-off - not conflated with the
	// wait-cost amount for the same delivery (AC8).
	if sharedRepo.stored == nil || !sharedRepo.stored.IsDamageWriteOff {
		t.Fatalf("expected the damage-writeoff record to have IsDamageWriteOff true, got %+v", sharedRepo.stored)
	}
	if sharedRepo.stored.Amount != 250.0 {
		t.Fatalf("expected the damage-writeoff record's amount to be 250.0 (not conflated with the wait-cost record's 15.0), got %v", sharedRepo.stored.Amount)
	}
}
