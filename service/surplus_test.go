package service

import (
	"errors"
	"testing"

	"github.com/ngolik/expense-service/model"
)

func TestAddSurplus(t *testing.T) {
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
			err := AddSurplus(&expense, tt.validator, repo)

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
				t.Fatalf("expected the generated id to be visible on the caller's expense after AddSurplus returns, got 0")
			}
			if tt.wantCreated && !expense.IsSurplus {
				t.Fatalf("expected the created record to have IsSurplus == true, got false")
			}
			if !tt.wantCreated && tt.wantErrType != "" && len(repo.records) != 0 {
				t.Fatalf("expected no record to be created on rejection, got %d", len(repo.records))
			}
		})
	}
}

func TestGetSurplus(t *testing.T) {
	repo := newFakeExpenseRepository()
	repo.records[1] = &model.Expense{DeliveryID: 100, UserID: 7, Amount: 42.5, IsSurplus: true}
	repo.records[1].ID = 1

	t.Run("found", func(t *testing.T) {
		got, err := GetSurplus(1, repo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.DeliveryID != 100 || got.UserID != 7 || got.Amount != 42.5 || !got.IsSurplus {
			t.Fatalf("unexpected record: %+v", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := GetSurplus(999, repo)
		if err == nil {
			t.Fatalf("expected an error for a missing record, got nil")
		}
	})
}

// TestAC1_SurplusDistinguishableFromWaitCostDamageWriteOffAndShortage
// creates a wait-cost record, a damage-writeoff record, a shortage record,
// and a surplus record for the SAME DeliveryID in the same repository, then
// fetches all four back and asserts IsSurplus/IsShortage/IsDamageWriteOff
// correctly distinguish them - finance must never see a surplus amount
// conflated with a wait-cost, damage-writeoff, or shortage amount for the
// same delivery (spec AC1).
func TestAC1_SurplusDistinguishableFromWaitCostDamageWriteOffAndShortage(t *testing.T) {
	repo := newFakeExpenseRepository()
	validator := &fakeUserValidator{exists: true}

	waitCost := model.Expense{DeliveryID: 400, UserID: 7, Amount: 15.0}
	if err := AddWaitingDeliveryCost(&waitCost, validator, repo); err != nil {
		t.Fatalf("unexpected error creating wait-cost record: %v", err)
	}

	writeOff := model.Expense{DeliveryID: 400, UserID: 9, Amount: 250.0}
	if err := AddDamageWriteOff(&writeOff, validator, repo); err != nil {
		t.Fatalf("unexpected error creating damage-writeoff record: %v", err)
	}

	shortage := model.Expense{DeliveryID: 400, UserID: 11, Amount: 75.0}
	if err := AddShortage(&shortage, validator, repo); err != nil {
		t.Fatalf("unexpected error creating shortage record: %v", err)
	}

	surplus := model.Expense{DeliveryID: 400, UserID: 13, Amount: 30.0}
	if err := AddSurplus(&surplus, validator, repo); err != nil {
		t.Fatalf("unexpected error creating surplus record: %v", err)
	}

	gotWaitCost, err := GetWaitingDeliveryCost(waitCost.ID, repo)
	if err != nil {
		t.Fatalf("unexpected error fetching wait-cost record: %v", err)
	}
	if gotWaitCost.IsShortage || gotWaitCost.IsDamageWriteOff || gotWaitCost.IsSurplus {
		t.Fatalf("expected the wait-cost record's IsShortage/IsDamageWriteOff/IsSurplus to be false, got %+v", gotWaitCost)
	}

	gotWriteOff, err := GetDamageWriteOff(writeOff.ID, repo)
	if err != nil {
		t.Fatalf("unexpected error fetching damage-writeoff record: %v", err)
	}
	if gotWriteOff.IsShortage || !gotWriteOff.IsDamageWriteOff || gotWriteOff.IsSurplus {
		t.Fatalf("expected the damage-writeoff record's IsShortage/IsSurplus false and IsDamageWriteOff true, got %+v", gotWriteOff)
	}

	gotShortage, err := GetShortage(shortage.ID, repo)
	if err != nil {
		t.Fatalf("unexpected error fetching shortage record: %v", err)
	}
	if !gotShortage.IsShortage || gotShortage.IsDamageWriteOff || gotShortage.IsSurplus {
		t.Fatalf("expected the shortage record's IsShortage true and IsDamageWriteOff/IsSurplus false, got %+v", gotShortage)
	}

	gotSurplus, err := GetSurplus(surplus.ID, repo)
	if err != nil {
		t.Fatalf("unexpected error fetching surplus record: %v", err)
	}
	if !gotSurplus.IsSurplus || gotSurplus.IsShortage || gotSurplus.IsDamageWriteOff {
		t.Fatalf("expected the surplus record's IsSurplus true and IsShortage/IsDamageWriteOff false, got %+v", gotSurplus)
	}
	if gotSurplus.DeliveryID != 400 {
		t.Fatalf("expected DeliveryID 400 on the surplus record, got %d", gotSurplus.DeliveryID)
	}

	ids := map[uint]bool{gotWaitCost.ID: true, gotWriteOff.ID: true, gotShortage.ID: true, gotSurplus.ID: true}
	if len(ids) != 4 {
		t.Fatalf("expected 4 distinct record ids, got %d: %d/%d/%d/%d", len(ids), gotWaitCost.ID, gotWriteOff.ID, gotShortage.ID, gotSurplus.ID)
	}
}
