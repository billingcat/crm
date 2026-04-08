package model_test

import (
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
)

func TestAuditLog_CreateAndList(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	ownerID := fixtures.DefaultOwnerID
	userID := data.User.ID

	// Create some audit entries
	store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityCompany, data.Company.ID, "Test GmbH")
	store.LogAudit(ownerID, userID, model.AuditActionLogin, model.AuditEntityUser, userID, "test@example.com")
	store.LogAudit(ownerID, userID, model.AuditActionUpdate, model.AuditEntityInvoice, data.Invoice.ID, "INV-001")

	// List all
	entries, total, err := store.ListAuditLogs(ownerID, model.AuditLogFilter{}, 0, 50)
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 entries, got %d", total)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries returned, got %d", len(entries))
	}

	// Newest first
	if entries[0].Action != model.AuditActionUpdate {
		t.Errorf("expected newest entry to be 'update', got %q", entries[0].Action)
	}

	// Check joined user data
	if entries[0].UserEmail == "" {
		t.Error("expected UserEmail to be populated")
	}
}

func TestAuditLog_FilterByAction(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	ownerID := fixtures.DefaultOwnerID
	userID := data.User.ID

	store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityCompany, 1, "A")
	store.LogAudit(ownerID, userID, model.AuditActionLogin, model.AuditEntityUser, userID, "B")
	store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityPerson, 2, "C")

	action := model.AuditActionCreate
	entries, total, err := store.ListAuditLogs(ownerID, model.AuditLogFilter{Action: &action}, 0, 50)
	if err != nil {
		t.Fatalf("ListAuditLogs with filter failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 create entries, got %d", total)
	}
	for _, e := range entries {
		if e.Action != model.AuditActionCreate {
			t.Errorf("expected action 'create', got %q", e.Action)
		}
	}
}

func TestAuditLog_FilterByEntityType(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	ownerID := fixtures.DefaultOwnerID
	userID := data.User.ID

	store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityCompany, 1, "A")
	store.LogAudit(ownerID, userID, model.AuditActionUpdate, model.AuditEntityCompany, 1, "B")
	store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityInvoice, 2, "C")

	et := model.AuditEntityCompany
	entries, total, err := store.ListAuditLogs(ownerID, model.AuditLogFilter{EntityType: &et}, 0, 50)
	if err != nil {
		t.Fatalf("ListAuditLogs with entity filter failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 company entries, got %d", total)
	}
	for _, e := range entries {
		if e.EntityType != model.AuditEntityCompany {
			t.Errorf("expected entity type 'company', got %q", e.EntityType)
		}
	}
}

func TestAuditLog_Pagination(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	ownerID := fixtures.DefaultOwnerID
	userID := data.User.ID

	for i := 0; i < 10; i++ {
		store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityCompany, uint(i+1), "Entry")
	}

	// Page 1
	entries, total, err := store.ListAuditLogs(ownerID, model.AuditLogFilter{}, 0, 3)
	if err != nil {
		t.Fatalf("ListAuditLogs page 1 failed: %v", err)
	}
	if total != 10 {
		t.Fatalf("expected total 10, got %d", total)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries on page 1, got %d", len(entries))
	}

	// Page 2
	entries2, _, err := store.ListAuditLogs(ownerID, model.AuditLogFilter{}, 3, 3)
	if err != nil {
		t.Fatalf("ListAuditLogs page 2 failed: %v", err)
	}
	if len(entries2) != 3 {
		t.Fatalf("expected 3 entries on page 2, got %d", len(entries2))
	}
	if entries[0].ID == entries2[0].ID {
		t.Error("page 1 and page 2 should return different entries")
	}
}

func TestAuditLog_ListUsers(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	ownerID := fixtures.DefaultOwnerID
	userID := data.User.ID

	store.LogAudit(ownerID, userID, model.AuditActionLogin, model.AuditEntityUser, userID, "login")

	users, err := store.ListAuditLogUsers(ownerID)
	if err != nil {
		t.Fatalf("ListAuditLogUsers failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].ID != userID {
		t.Errorf("expected user ID %d, got %d", userID, users[0].ID)
	}
}

func TestAuditLog_OwnerIsolation(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	ownerID := fixtures.DefaultOwnerID
	userID := data.User.ID

	store.LogAudit(ownerID, userID, model.AuditActionCreate, model.AuditEntityCompany, 1, "Owner 1")
	store.LogAudit(999, userID, model.AuditActionCreate, model.AuditEntityCompany, 2, "Owner 999")

	entries, total, err := store.ListAuditLogs(ownerID, model.AuditLogFilter{}, 0, 50)
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 entry for owner %d, got %d", ownerID, total)
	}
	if entries[0].Summary != "Owner 1" {
		t.Errorf("expected summary 'Owner 1', got %q", entries[0].Summary)
	}
}
