package model_test

import (
	"context"
	"errors"
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
)

const (
	tenantA uint = 1
	tenantB uint = 2
	tenantC uint = 3 // has no settings row
)

// seedTwoTenants creates settings for tenant A (prefix "A-", width 4, counter 10)
// and tenant B (prefix "B-", width 3, counter 5). Tenant A's row is inserted
// first, so an unscoped First() would always hit it.
func seedTwoTenants(t *testing.T) *model.Store {
	t.Helper()
	store := fixtures.NewTestStore(t)

	a := fixtures.Settings(fixtures.WithSettingsOwnerID(tenantA))
	a.CustomerNumberPrefix, a.CustomerNumberWidth, a.CustomerNumberCounter = "A-", 4, 10
	b := fixtures.Settings(fixtures.WithSettingsOwnerID(tenantB))
	b.CustomerNumberPrefix, b.CustomerNumberWidth, b.CustomerNumberCounter = "B-", 3, 5

	for _, s := range []*model.Settings{a, b} {
		if err := store.SaveSettings(s); err != nil {
			t.Fatalf("SaveSettings(owner %d) failed: %v", s.OwnerID, err)
		}
	}
	return store
}

func saveCompanyWithNumber(t *testing.T, store *model.Store, ownerID uint, num string) *model.Company {
	t.Helper()
	c := fixtures.Company(
		fixtures.WithCompanyOwnerID(ownerID),
		fixtures.WithCompanyCustomerNumber(num),
	)
	if err := store.SaveCompany(c, ownerID, nil); err != nil {
		t.Fatalf("SaveCompany(owner %d, %q) failed: %v", ownerID, num, err)
	}
	return c
}

func customerCounter(t *testing.T, store *model.Store, ownerID uint) int64 {
	t.Helper()
	s, err := store.LoadSettings(ownerID)
	if err != nil {
		t.Fatalf("LoadSettings(%d) failed: %v", ownerID, err)
	}
	return s.CustomerNumberCounter
}

func TestSuggestNextCustomerNumber_UsesOwnSettings(t *testing.T) {
	store := seedTwoTenants(t)
	ctx := context.Background()

	for _, tc := range []struct {
		owner uint
		want  string
	}{
		{tenantA, "A-0011"},
		{tenantB, "B-006"},
	} {
		got, err := store.SuggestNextCustomerNumber(ctx, tc.owner)
		if err != nil {
			t.Fatalf("SuggestNextCustomerNumber(%d) failed: %v", tc.owner, err)
		}
		if got != tc.want {
			t.Errorf("SuggestNextCustomerNumber(%d) = %q, want %q", tc.owner, got, tc.want)
		}
	}

	if _, err := store.SuggestNextCustomerNumber(ctx, tenantC); !errors.Is(err, model.ErrNoSettingsRow) {
		t.Errorf("SuggestNextCustomerNumber(owner without settings) err = %v, want ErrNoSettingsRow", err)
	}
}

func TestCheckCustomerNumber_UsesOwnRules(t *testing.T) {
	store := seedTwoTenants(t)
	ctx := context.Background()

	// Valid for A, but B's rules require prefix "B-".
	ok, _, err := store.CheckCustomerNumber(ctx, tenantB, "A-0011", 0)
	if err != nil {
		t.Fatalf("CheckCustomerNumber failed: %v", err)
	}
	if ok {
		t.Error("tenant B accepted a number that only matches tenant A's rules")
	}

	ok, msg, err := store.CheckCustomerNumber(ctx, tenantB, "B-042", 0)
	if err != nil {
		t.Fatalf("CheckCustomerNumber failed: %v", err)
	}
	if !ok {
		t.Errorf("tenant B rejected a number matching its own rules: %q", msg)
	}

	if _, _, err := store.CheckCustomerNumber(ctx, tenantC, "X-1", 0); err == nil {
		t.Error("CheckCustomerNumber for owner without settings should fail")
	}
}

func TestCheckCustomerNumber_UniquenessIsPerOwner(t *testing.T) {
	store := seedTwoTenants(t)
	ctx := context.Background()

	// Tenant A owns a company whose number would also be valid for B.
	compA := saveCompanyWithNumber(t, store, tenantA, "B-001")

	// B must not learn that another tenant uses this number.
	ok, msg, err := store.CheckCustomerNumber(ctx, tenantB, "B-001", 0)
	if err != nil {
		t.Fatalf("CheckCustomerNumber failed: %v", err)
	}
	if !ok {
		t.Errorf("tenant B sees tenant A's customer number as taken: %q", msg)
	}

	// Within the same tenant the number is taken ...
	saveCompanyWithNumber(t, store, tenantA, "A-0042")
	ok, _, err = store.CheckCustomerNumber(ctx, tenantA, "A-0042", 0)
	if err != nil {
		t.Fatalf("CheckCustomerNumber failed: %v", err)
	}
	if ok {
		t.Error("tenant A's own duplicate customer number was accepted")
	}

	// ... and passing another tenant's company ID as excludeID changes nothing.
	saveCompanyWithNumber(t, store, tenantB, "B-002")
	ok, _, err = store.CheckCustomerNumber(ctx, tenantB, "B-002", compA.ID)
	if err != nil {
		t.Fatalf("CheckCustomerNumber failed: %v", err)
	}
	if ok {
		t.Error("excludeID of a foreign company made a taken number available")
	}
}

func TestNextCustomerNumberTx_ScopedToOwner(t *testing.T) {
	store := seedTwoTenants(t)
	ctx := context.Background()

	// A foreign company with B's next number must not make B skip it.
	saveCompanyWithNumber(t, store, tenantA, "B-006")

	num, n, err := store.NextCustomerNumberTx(ctx, tenantB)
	if err != nil {
		t.Fatalf("NextCustomerNumberTx failed: %v", err)
	}
	if num != "B-006" || n != 6 {
		t.Errorf("NextCustomerNumberTx(B) = %q, %d; want %q, 6", num, n, "B-006")
	}
	if got := customerCounter(t, store, tenantB); got != 6 {
		t.Errorf("tenant B counter = %d, want 6", got)
	}
	if got := customerCounter(t, store, tenantA); got != 10 {
		t.Errorf("tenant A counter changed to %d, want 10", got)
	}

	// Own numbers are still skipped.
	saveCompanyWithNumber(t, store, tenantB, "B-007")
	num, _, err = store.NextCustomerNumberTx(ctx, tenantB)
	if err != nil {
		t.Fatalf("NextCustomerNumberTx failed: %v", err)
	}
	if num != "B-008" {
		t.Errorf("NextCustomerNumberTx(B) = %q, want %q", num, "B-008")
	}

	if _, _, err := store.NextCustomerNumberTx(ctx, tenantC); !errors.Is(err, model.ErrNoSettingsRow) {
		t.Errorf("NextCustomerNumberTx(owner without settings) err = %v, want ErrNoSettingsRow", err)
	}
}

func TestMaybeLiftCustomerCounterFor_ScopedToOwner(t *testing.T) {
	store := seedTwoTenants(t)
	ctx := context.Background()

	if err := store.MaybeLiftCustomerCounterFor(ctx, tenantB, "B-050"); err != nil {
		t.Fatalf("MaybeLiftCustomerCounterFor failed: %v", err)
	}
	if got := customerCounter(t, store, tenantB); got != 50 {
		t.Errorf("tenant B counter = %d, want 50", got)
	}
	if got := customerCounter(t, store, tenantA); got != 10 {
		t.Errorf("tenant A counter changed to %d, want 10", got)
	}

	// A number in A's format saved by B must not lift A's counter.
	if err := store.MaybeLiftCustomerCounterFor(ctx, tenantB, "A-9999"); err != nil {
		t.Fatalf("MaybeLiftCustomerCounterFor failed: %v", err)
	}
	if got := customerCounter(t, store, tenantA); got != 10 {
		t.Errorf("tenant A counter changed to %d, want 10", got)
	}
}
