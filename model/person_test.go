package model_test

import (
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
)

func TestPerson_DepartPerson(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Person from SeedTestData is linked to company
	result, err := store.DepartPerson(data.Person.ID, fixtures.DefaultOwnerID, data.User.ID)
	if err != nil {
		t.Fatalf("DepartPerson failed: %v", err)
	}

	// Check person is marked as departed
	if result.Person.DepartedAt == nil {
		t.Error("DepartedAt should be set")
	}
	if !result.Person.HasDeparted() {
		t.Error("HasDeparted() should return true")
	}

	// Check note was created
	if result.Note == nil {
		t.Fatal("Note should be created")
	}
	if result.Note.ParentType != model.ParentTypeCompany {
		t.Errorf("Note.ParentType = %q, want %q", result.Note.ParentType, model.ParentTypeCompany)
	}
	if result.Note.ParentID != uint(data.Person.CompanyID) {
		t.Errorf("Note.ParentID = %d, want %d", result.Note.ParentID, data.Person.CompanyID)
	}
	if result.Note.Title != "Mitarbeiter ausgeschieden" {
		t.Errorf("Note.Title = %q, want %q", result.Note.Title, "Mitarbeiter ausgeschieden")
	}

	// Verify note is in database and linked to company
	notes, err := store.LoadAllNotesForParent(fixtures.DefaultOwnerID, model.ParentTypeCompany, data.Company.ID)
	if err != nil {
		t.Fatalf("LoadAllNotesForParent failed: %v", err)
	}
	found := false
	for _, n := range notes {
		if n.ID == result.Note.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Note should be linked to company")
	}

	// Verify person still has CompanyID (for history)
	loaded, _ := store.LoadPerson(data.Person.ID, fixtures.DefaultOwnerID)
	if loaded.CompanyID == 0 {
		t.Error("Person should still be linked to company after departure")
	}
}

func TestPerson_DepartPerson_AlreadyDeparted(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Depart once
	_, err := store.DepartPerson(data.Person.ID, fixtures.DefaultOwnerID, data.User.ID)
	if err != nil {
		t.Fatalf("First DepartPerson failed: %v", err)
	}

	// Try to depart again
	_, err = store.DepartPerson(data.Person.ID, fixtures.DefaultOwnerID, data.User.ID)
	if err == nil {
		t.Error("Expected error for already departed person")
	}
}

func TestPerson_DepartPerson_NoCompany(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Create person without company
	person := fixtures.Person(fixtures.WithPersonCompanyID(0))
	if err := store.SavePerson(person, fixtures.DefaultOwnerID, nil); err != nil {
		t.Fatalf("SavePerson failed: %v", err)
	}

	// Try to depart
	_, err := store.DepartPerson(person.ID, fixtures.DefaultOwnerID, data.User.ID)
	if err == nil {
		t.Error("Expected error for person without company")
	}
}

func TestPerson_ReactivatePerson(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Depart person
	_, err := store.DepartPerson(data.Person.ID, fixtures.DefaultOwnerID, data.User.ID)
	if err != nil {
		t.Fatalf("DepartPerson failed: %v", err)
	}

	// Verify departed
	loaded, _ := store.LoadPerson(data.Person.ID, fixtures.DefaultOwnerID)
	if !loaded.HasDeparted() {
		t.Fatal("Person should be departed")
	}

	// Reactivate
	if err := store.ReactivatePerson(data.Person.ID, fixtures.DefaultOwnerID); err != nil {
		t.Fatalf("ReactivatePerson failed: %v", err)
	}

	// Verify reactivated
	loaded, _ = store.LoadPerson(data.Person.ID, fixtures.DefaultOwnerID)
	if loaded.HasDeparted() {
		t.Error("Person should not be departed after reactivation")
	}
}

func TestPerson_HasDeparted(t *testing.T) {
	p1 := &model.Person{}
	if p1.HasDeparted() {
		t.Error("New person should not be departed")
	}

	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	store.DepartPerson(data.Person.ID, fixtures.DefaultOwnerID, data.User.ID)
	loaded, _ := store.LoadPerson(data.Person.ID, fixtures.DefaultOwnerID)

	if !loaded.HasDeparted() {
		t.Error("Departed person should return HasDeparted() = true")
	}
}
