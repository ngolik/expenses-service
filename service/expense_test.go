package service

import (
	"errors"
	"testing"

	"github.com/ngolik/expense-service/model"
	"gorm.io/gorm"
)

// fakeRepository is an in-memory userExpenseRepository used to unit test
// AddExpense/GetExpenseById without a live database connection.
type fakeRepository struct {
	users    map[int]bool
	expenses map[uint]model.Expense
	nextID   uint
}

func newFakeRepository(knownUserIDs ...int) *fakeRepository {
	users := make(map[int]bool, len(knownUserIDs))
	for _, id := range knownUserIDs {
		users[id] = true
	}
	return &fakeRepository{users: users, expenses: make(map[uint]model.Expense)}
}

func (f *fakeRepository) UserExists(userID int) (bool, error) {
	return f.users[userID], nil
}

func (f *fakeRepository) CreateExpense(expense model.Expense) error {
	f.nextID++
	expense.ID = f.nextID
	f.expenses[expense.ID] = expense
	return nil
}

func (f *fakeRepository) FindExpenseByID(id uint) (model.Expense, error) {
	expense, ok := f.expenses[id]
	if !ok {
		return model.Expense{}, gorm.ErrRecordNotFound
	}
	return expense, nil
}

func withFakeRepo(t *testing.T, repo *fakeRepository) {
	t.Helper()
	previous := Repo
	Repo = repo
	t.Cleanup(func() { Repo = previous })
}

func TestAddExpense_ValidUserID(t *testing.T) {
	repo := newFakeRepository(42)
	withFakeRepo(t, repo)

	err := AddExpense(model.Expense{UserID: 42, Description: "taxi"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.expenses) != 1 {
		t.Fatalf("expected 1 expense to be persisted, got %d", len(repo.expenses))
	}
}

func TestAddExpense_OmittedUserID(t *testing.T) {
	repo := newFakeRepository(42)
	withFakeRepo(t, repo)

	err := AddExpense(model.Expense{UserID: 0, Description: "lunch"})
	if err != nil {
		t.Fatalf("expected no error for omitted UserID, got %v", err)
	}
	if len(repo.expenses) != 1 {
		t.Fatalf("expected 1 expense to be persisted, got %d", len(repo.expenses))
	}
}

func TestAddExpense_UnknownUserID(t *testing.T) {
	repo := newFakeRepository(42)
	withFakeRepo(t, repo)

	err := AddExpense(model.Expense{UserID: 999, Description: "hotel"})
	if !errors.Is(err, ErrUnknownUser) {
		t.Fatalf("expected ErrUnknownUser, got %v", err)
	}
	if len(repo.expenses) != 0 {
		t.Fatalf("expected no expense to be persisted, got %d", len(repo.expenses))
	}
}

func TestGetExpenseById_EchoesStoredUserID(t *testing.T) {
	repo := newFakeRepository(7)
	withFakeRepo(t, repo)

	if err := AddExpense(model.Expense{UserID: 7, Description: "flight"}); err != nil {
		t.Fatalf("setup: AddExpense failed: %v", err)
	}

	got, err := GetExpenseById(1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.UserID != 7 {
		t.Fatalf("expected echoed UserID 7, got %d", got.UserID)
	}
}

func TestGetExpenseById_NotFound(t *testing.T) {
	withFakeRepo(t, newFakeRepository())

	_, err := GetExpenseById(999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}
