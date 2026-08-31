package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ngolik/expense-service/model"
	"gorm.io/gorm"
)

// fakeValidator and waitingCostFakeRepo let these handler tests run without a live
// auth-service or database, substituted in place of the package's default
// waitingDeliveryCostValidator/waitingDeliveryCostRepo vars.

type fakeValidator struct {
	exists bool
	err    error
}

func (f *fakeValidator) UserExists(userID int) (bool, error) {
	return f.exists, f.err
}

type waitingCostFakeRepo struct {
	createErr error
	findErr   error
	stored    *model.Expense
}

func (f *waitingCostFakeRepo) Create(expense *model.Expense) error {
	if f.createErr != nil {
		return f.createErr
	}
	expense.ID = 1
	f.stored = expense
	return nil
}

func (f *waitingCostFakeRepo) FindByID(id uint) (*model.Expense, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.stored == nil || f.stored.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.stored, nil
}

func setupRouterForTest() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router)
	return router
}

func performRequest(router *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAddWaitingDeliveryCostHandler(t *testing.T) {
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
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 99.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 200,
		},
		{
			name:       "missing delivery id - 400",
			body:       reqBody{UserID: 5, Amount: 99.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "missing user id - 400",
			body:       reqBody{DeliveryID: 10, Amount: 99.99},
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
			body:       reqBody{DeliveryID: 10, UserID: 999, Amount: 99.99},
			validator:  &fakeValidator{exists: false},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 400,
		},
		{
			name:       "auth-service call fails - 502",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 99.99},
			validator:  &fakeValidator{err: errors.New("connection refused")},
			repo:       &waitingCostFakeRepo{},
			wantStatus: 502,
		},
		{
			name:       "db create fails - 500",
			body:       reqBody{DeliveryID: 10, UserID: 5, Amount: 99.99},
			validator:  &fakeValidator{exists: true},
			repo:       &waitingCostFakeRepo{createErr: errors.New("db down")},
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalValidator := waitingDeliveryCostValidator
			originalRepo := waitingDeliveryCostRepo
			waitingDeliveryCostValidator = tt.validator
			waitingDeliveryCostRepo = tt.repo
			defer func() {
				waitingDeliveryCostValidator = originalValidator
				waitingDeliveryCostRepo = originalRepo
			}()

			router := setupRouterForTest()
			w := performRequest(router, http.MethodPost, "/expenses/rest/waiting-cost", tt.body)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == 200 {
				var resp WaitingDeliveryCostResponse
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

func TestAddWaitingDeliveryCostHandler_MalformedBody(t *testing.T) {
	originalValidator := waitingDeliveryCostValidator
	originalRepo := waitingDeliveryCostRepo
	waitingDeliveryCostValidator = &fakeValidator{exists: true}
	waitingDeliveryCostRepo = &waitingCostFakeRepo{}
	defer func() {
		waitingDeliveryCostValidator = originalValidator
		waitingDeliveryCostRepo = originalRepo
	}()

	router := setupRouterForTest()
	req := httptest.NewRequest(http.MethodPost, "/expenses/rest/waiting-cost", bytes.NewBufferString("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestGetWaitingDeliveryCostHandler(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{stored: &model.Expense{DeliveryID: 10, UserID: 5, Amount: 99.99}}
		repo.stored.ID = 1

		originalRepo := waitingDeliveryCostRepo
		waitingDeliveryCostRepo = repo
		defer func() { waitingDeliveryCostRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/waiting-cost/1", nil)

		if w.Code != 200 {
			t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
		}

		var resp WaitingDeliveryCostResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp.DeliveryID != 10 || resp.UserID != 5 || resp.Amount != 99.99 {
			t.Fatalf("unexpected response body: %+v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := waitingDeliveryCostRepo
		waitingDeliveryCostRepo = repo
		defer func() { waitingDeliveryCostRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/waiting-cost/999", nil)

		if w.Code != 404 {
			t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-numeric id", func(t *testing.T) {
		repo := &waitingCostFakeRepo{}
		originalRepo := waitingDeliveryCostRepo
		waitingDeliveryCostRepo = repo
		defer func() { waitingDeliveryCostRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/waiting-cost/abc", nil)

		if w.Code != 400 {
			t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("other repository error - 500", func(t *testing.T) {
		repo := &waitingCostFakeRepo{findErr: errors.New("db exploded")}
		originalRepo := waitingDeliveryCostRepo
		waitingDeliveryCostRepo = repo
		defer func() { waitingDeliveryCostRepo = originalRepo }()

		router := setupRouterForTest()
		w := performRequest(router, http.MethodGet, "/expenses/rest/waiting-cost/1", nil)

		if w.Code != 500 {
			t.Fatalf("status = %d, want 500, body = %s", w.Code, w.Body.String())
		}
	})
}
