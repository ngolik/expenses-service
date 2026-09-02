package service

import (
	"errors"
	"testing"

	"github.com/ngolik/expense-service/model"
)

func TestAddDamageWriteOff(t *testing.T) {
	baseExpense := func() model.Expense {
		return model.Expense{DeliveryID: 100, UserID: 7, Amount: 42.5}
	}

	tests := []struct {
		name          string
		mutate        func(model.Expense) model.Expense
		validator     *fakeUserValidator
		repoCreateErr error
		wantErrType   string // "", "validation", "upstream", "other"
		wantCreated   bool
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
			err := AddDamageWriteOff(&expense, tt.validator, repo)

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
				t.Fatalf("expected the generated id to be visible on the caller's expense after AddDamageWriteOff returns, got 0")
			}
			if tt.wantCreated && !expense.IsDamageWriteOff {
				t.Fatalf("expected the created record to have IsDamageWriteOff == true, got false")
			}
			if !tt.wantCreated && tt.wantErrType != "" && len(repo.records) != 0 {
				t.Fatalf("expected no record to be created on rejection, got %d", len(repo.records))
			}
		})
	}
}

func TestGetDamageWriteOff(t *testing.T) {
	repo := newFakeExpenseRepository()
	repo.records[1] = &model.Expense{DeliveryID: 100, UserID: 7, Amount: 42.5, IsDamageWriteOff: true}
	repo.records[1].ID = 1

	t.Run("found", func(t *testing.T) {
		got, err := GetDamageWriteOff(1, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.DeliveryID != 100 || got.UserID != 7 || got.Amount != 42.5 || !got.IsDamageWriteOff {
			t.Fatalf("unexpected record: %+v", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetDamageWriteOff(999, repo)
		if err == nil {
			t.Fatalf("expected an error for a missing record, got nil")
		}
	})
}

// TestAC8_WaitCostAndDamageWriteOffAreDistinguishable creates a wait-cost
// record and a damage-writeoff record for the SAME DeliveryID in the same
// repository, then fetches both back and asserts IsDamageWriteOff correctly
// distinguishes them - finance must never see a damage write-off conflated
// with a wait-cost amount for the same delivery (AC8).
func TestAC8_WaitCostAndDamageWriteOffAreDistinguishable(t *testing.T) {
	repo := newFakeExpenseRepository()
	validator := &fakeUserValidator{exists: true}

	waitCost := model.Expense{DeliveryID: 200, UserID: 7, Amount: 15.0}
	if err := AddWaitingDeliveryCost(&waitCost, validator, repo); err != nil {
		t.Fatalf("unexpected error creating wait-cost record: %v", err)
	}

	writeOff := model.Expense{DeliveryID: 200, UserID: 9, Amount: 250.0}
	if err := AddDamageWriteOff(&writeOff, validator, repo); err != nil {
		t.Fatalf("unexpected error creating damage-writeoff record: %v", err)
	}

	gotWaitCost, err := GetWaitingDeliveryCost(waitCost.ID, repo)
	if err != nil {
		t.Fatalf("unexpected error fetching wait-cost record: %v", err)
	}
	if gotWaitCost.IsDamageWriteOff {
		t.Fatalf("expected the wait-cost record's IsDamageWriteOff to be false, got true: %+v", gotWaitCost)
	}
	if gotWaitCost.DeliveryID != 200 {
		t.Fatalf("expected DeliveryID 200 on the wait-cost record, got %d", gotWaitCost.DeliveryID)
	}

	gotWriteOff, err := GetDamageWriteOff(writeOff.ID, repo)
	if err != nil {
		t.Fatalf("unexpected error fetching damage-writeoff record: %v", err)
	}
	if !gotWriteOff.IsDamageWriteOff {
		t.Fatalf("expected the damage-writeoff record's IsDamageWriteOff to be true, got false: %+v", gotWriteOff)
	}
	if gotWriteOff.DeliveryID != 200 {
		t.Fatalf("expected DeliveryID 200 on the damage-writeoff record, got %d", gotWriteOff.DeliveryID)
	}

	if gotWaitCost.ID == gotWriteOff.ID {
		t.Fatalf("expected distinct record ids for the two records, both got %d", gotWaitCost.ID)
	}
}
