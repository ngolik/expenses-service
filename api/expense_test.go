package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ngolik/expense-service/model"
	"github.com/ngolik/expense-service/service"
	"gorm.io/gorm"
)

// fakeRepo is a minimal stand-in for the service package's unexported
// userExpenseRepository, used to drive the HTTP handlers without a database.
type fakeRepo struct {
	knownUserIDs map[int]bool
	expenses     map[uint]model.Expense
}

func (f fakeRepo) UserExists(userID int) (bool, error) {
	return f.knownUserIDs[userID], nil
}

func (f fakeRepo) CreateExpense(expense model.Expense) error {
	return nil
}

func (f fakeRepo) FindExpenseByID(id uint) (model.Expense, error) {
	expense, ok := f.expenses[id]
	if !ok {
		return model.Expense{}, gorm.ErrRecordNotFound
	}
	return expense, nil
}

func withFakeServiceRepo(t *testing.T, repo fakeRepo) {
	t.Helper()
	previous := service.Repo
	service.Repo = repo
	t.Cleanup(func() { service.Repo = previous })
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router)
	return router
}

func TestAddExpenseHandler_UnknownUser_Returns400(t *testing.T) {
	withFakeServiceRepo(t, fakeRepo{knownUserIDs: map[int]bool{}})
	router := newTestRouter()

	body, _ := json.Marshal(model.Expense{UserID: 999, Description: "hotel"})
	req := httptest.NewRequest(http.MethodPost, "/expenses/rest/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestAddExpenseHandler_OmittedUser_Returns200(t *testing.T) {
	withFakeServiceRepo(t, fakeRepo{knownUserIDs: map[int]bool{}})
	router := newTestRouter()

	body, _ := json.Marshal(model.Expense{Description: "lunch"})
	req := httptest.NewRequest(http.MethodPost, "/expenses/rest/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestGetExpenseByIdHandler_NotFound_Returns404(t *testing.T) {
	withFakeServiceRepo(t, fakeRepo{expenses: map[uint]model.Expense{}})
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/expenses/rest/999", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestGetExpenseByIdHandler_EchoesUserID_Returns200(t *testing.T) {
	stored := model.Expense{UserID: 7, Description: "flight"}
	stored.ID = 1
	withFakeServiceRepo(t, fakeRepo{expenses: map[uint]model.Expense{1: stored}})
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/expenses/rest/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var got model.Expense
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.UserID != 7 {
		t.Fatalf("expected echoed UserID 7, got %d", got.UserID)
	}
}
