package service

import (
	"errors"
	"testing"

	"github.com/ngolik/expense-service/model"
)

// fakeExpenseRepository is an in-memory ExpenseRepository used only in
// tests, so the wait-cost business logic can be verified without a live
// database.
type fakeExpenseRepository struct {
	records   map[uint]*model.Expense
	nextID    uint
	createErr error
}

func newFakeExpenseRepository() *fakeExpenseRepository {
	return &fakeExpenseRepository{records: make(map[uint]*model.Expense)}
}

func (f *fakeExpenseRepository) Create(expense *model.Expense) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	expense.ID = f.nextID
	stored := *expense
	f.records[expense.ID] = &stored
	return nil
}

func (f *fakeExpenseRepository) FindByID(id uint) (*model.Expense, error) {
	record, ok := f.records[id]
	if !ok {
		return nil, errRecordNotFoundForTest
	}
	return record, nil
}

var errRecordNotFoundForTest = errors.New("record not found")

// fakeUserValidator is a fake UserValidator used only in tests, in place of
// a live call to auth-service.
type fakeUserValidator struct {
	exists bool
	err    error
}

func (f *fakeUserValidator) UserExists(userID int) (bool, error) {
	return f.exists, f.err
}

func TestAddWaitingDeliveryCost(t *testing.T) {
	baseExpense := func() model.Expense {
		return model.Expense{DeliveryID: 100, UserID: 7, Amount: 42.5}
	}

	tests := []struct {
		name         string
		mutate       func(model.Expense) model.Expense
		validator    *fakeUserValidator
		repoCreateErr error
		wantErrType  string // "", "validation", "upstream", "other"
		wantCreated  bool
	}{
		{
			name:        "all fields present and user exists - created",
			mutate:      func(e model.Expense) model.Expense { return e },
			validator:   &fakeUserValidator{exists: true},
			wantErrType: "",
			wantCreated: true,
		},
		{
			name:        "missing delivery id is rejected",
			mutate:      func(e model.Expense) model.Expense { e.DeliveryID = 0; return e },
			validator:   &fakeUserValidator{exists: true},
			wantErrType: "validation",
		},
		{
			name:        "missing user id is rejected",
			mutate:      func(e model.Expense) model.Expense { e.UserID = 0; return e },
			validator:   &fakeUserValidator{exists: true},
			wantErrType: "validation",
		},
		{
			name:        "missing amount is rejected",
			mutate:      func(e model.Expense) model.Expense { e.Amount = 0; return e },
			validator:   &fakeUserValidator{exists: true},
			wantErrType: "validation",
		},
		{
			name:        "unknown user is rejected",
			mutate:      func(e model.Expense) model.Expense { return e },
			validator:   &fakeUserValidator{exists: false},
			wantErrType: "validation",
		},
		{
			name:        "auth-service call failure is an upstream error, not validation",
			mutate:      func(e model.Expense) model.Expense { return e },
			validator:   &fakeUserValidator{err: errors.New("connection refused")},
			wantErrType: "upstream",
		},
		{
			name:          "repository create failure surfaces as-is",
			mutate:        func(e model.Expense) model.Expense { return e },
			validator:     &fakeUserValidator{exists: true},
			repoCreateErr: errors.New("db is down"),
			wantErrType:   "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeExpenseRepository()
			repo.createErr = tt.repoCreateErr

			expense := tt.mutate(baseExpense())
			err := AddWaitingDeliveryCost(&expense, tt.validator, repo)

			switch tt.wantErrType {
			case "":
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			case "validation":
				var validationErr *ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("expected *ValidationError, got %T (%v)", err, err)
				}
			case "upstream":
				var upstreamErr *UpstreamError
				if !errors.As(err, &upstreamErr) {
					t.Fatalf("expected *UpstreamError, got %T (%v)", err, err)
				}
			case "other":
				var validationErr *ValidationError
				var upstreamErr *UpstreamError
				if errors.As(err, &validationErr) || errors.As(err, &upstreamErr) {
					t.Fatalf("expected a plain (non-validation, non-upstream) error, got %T (%v)", err, err)
				}
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
			}

			if tt.wantCreated && len(repo.records) != 1 {
				t.Fatalf("expected 1 record to be created, got %d", len(repo.records))
			}
			if tt.wantCreated && expense.ID == 0 {
				t.Fatalf("expected the generated id to be visible on the caller's expense after AddWaitingDeliveryCost returns, got 0")
			}
			if !tt.wantCreated && tt.wantErrType != "" && len(repo.records) != 0 {
				t.Fatalf("expected no record to be created on rejection, got %d", len(repo.records))
			}
		})
	}
}

func TestGetWaitingDeliveryCost(t *testing.T) {
	repo := newFakeExpenseRepository()
	repo.records[1] = &model.Expense{DeliveryID: 100, UserID: 7, Amount: 42.5}
	repo.records[1].ID = 1

	t.Run("found", func(t *testing.T) {
		got, err := GetWaitingDeliveryCost(1, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.DeliveryID != 100 || got.UserID != 7 || got.Amount != 42.5 {
			t.Fatalf("unexpected record: %+v", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetWaitingDeliveryCost(999, repo)
		if err == nil {
			t.Fatalf("expected an error for a missing record, got nil")
		}
	})
}
