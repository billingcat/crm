package model_test

import (
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
)

func TestNote_CreateAndLoad(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	note := fixtures.NoteForCompany(data.Company.ID,
		fixtures.WithNoteTitle("Wichtiger Hinweis"),
		fixtures.WithNoteBody("Kunde bevorzugt E-Mail-Kontakt."),
		fixtures.WithNoteTags("kommunikation,präferenz"),
	)

	if err := store.CreateNote(note); err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	if note.ID == 0 {
		t.Fatal("Note ID should be non-zero after create")
	}

	// Load it back
	loaded, err := store.GetNoteByID(note.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("GetNoteByID failed: %v", err)
	}

	if loaded.Title != "Wichtiger Hinweis" {
		t.Errorf("Title = %q, want %q", loaded.Title, "Wichtiger Hinweis")
	}
	if loaded.Body != "Kunde bevorzugt E-Mail-Kontakt." {
		t.Errorf("Body mismatch")
	}
	if loaded.Tags != "kommunikation,präferenz" {
		t.Errorf("Tags = %q, want %q", loaded.Tags, "kommunikation,präferenz")
	}
	if loaded.EditedAt.IsZero() {
		t.Error("EditedAt should be set automatically")
	}
}

func TestNote_ListForParent(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Create multiple notes for the company
	for i, title := range []string{"Erste Notiz", "Zweite Notiz", "Dritte Notiz"} {
		note := fixtures.NoteForCompany(data.Company.ID,
			fixtures.WithNoteTitle(title),
			fixtures.WithNoteBody("Inhalt "+string(rune('A'+i))),
		)
		if err := store.CreateNote(note); err != nil {
			t.Fatalf("CreateNote %d failed: %v", i, err)
		}
	}

	// Create a note for a different parent (person)
	personNote := fixtures.NoteForPerson(data.Person.ID,
		fixtures.WithNoteTitle("Person Notiz"),
	)
	if err := store.CreateNote(personNote); err != nil {
		t.Fatalf("CreateNote for person failed: %v", err)
	}

	// Load notes for company
	companyNotes, err := store.LoadAllNotesForParent(
		fixtures.DefaultOwnerID,
		model.ParentTypeCompany,
		data.Company.ID,
	)
	if err != nil {
		t.Fatalf("LoadAllNotesForParent failed: %v", err)
	}

	if len(companyNotes) != 3 {
		t.Errorf("Company notes count = %d, want 3", len(companyNotes))
	}

	// Load notes for person
	personNotes, err := store.LoadAllNotesForParent(
		fixtures.DefaultOwnerID,
		model.ParentTypePerson,
		data.Person.ID,
	)
	if err != nil {
		t.Fatalf("LoadAllNotesForParent for person failed: %v", err)
	}

	if len(personNotes) != 1 {
		t.Errorf("Person notes count = %d, want 1", len(personNotes))
	}
}

func TestNote_UpdateAsAuthor(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	note := fixtures.NoteForCompany(data.Company.ID,
		fixtures.WithNoteTitle("Original"),
		fixtures.WithNoteBody("Ursprünglicher Text"),
		fixtures.WithNoteAuthorID(data.User.ID),
	)
	if err := store.CreateNote(note); err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	// Update as the author
	updated, err := store.UpdateNoteContentAsAuthor(
		fixtures.DefaultOwnerID,
		data.User.ID,
		note.ID,
		"Geändert",
		"Neuer Text",
		"neu,geändert",
	)
	if err != nil {
		t.Fatalf("UpdateNoteContentAsAuthor failed: %v", err)
	}

	if updated.Title != "Geändert" {
		t.Errorf("Title = %q, want %q", updated.Title, "Geändert")
	}
	if updated.Body != "Neuer Text" {
		t.Errorf("Body = %q, want %q", updated.Body, "Neuer Text")
	}
}

func TestNote_UpdateAsWrongAuthor_Forbidden(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Create second user
	otherUser := fixtures.User(fixtures.WithUserEmail("other@example.com"))
	if err := store.CreateUser(otherUser); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create note as first user
	note := fixtures.NoteForCompany(data.Company.ID,
		fixtures.WithNoteAuthorID(data.User.ID),
	)
	if err := store.CreateNote(note); err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	// Try to update as different user
	_, err := store.UpdateNoteContentAsAuthor(
		fixtures.DefaultOwnerID,
		otherUser.ID, // wrong author
		note.ID,
		"Hacked",
		"Sollte nicht funktionieren",
		"",
	)

	if err == nil {
		t.Fatal("Expected error when updating as wrong author, got nil")
	}
	if err.Error() != "forbidden" {
		t.Errorf("Error = %q, want %q", err.Error(), "forbidden")
	}
}

func TestNote_DeleteAsAuthor(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	note := fixtures.NoteForCompany(data.Company.ID,
		fixtures.WithNoteAuthorID(data.User.ID),
	)
	if err := store.CreateNote(note); err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	// Delete as author
	if err := store.DeleteNote(note.ID, fixtures.DefaultOwnerID, data.User.ID); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	// Verify it's gone
	_, err := store.GetNoteByID(note.ID, fixtures.DefaultOwnerID)
	if err == nil {
		t.Error("Note should be deleted, but GetNoteByID returned no error")
	}
}

func TestNote_DeleteAsWrongAuthor_NoEffect(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Create second user
	otherUser := fixtures.User(fixtures.WithUserEmail("other@example.com"))
	if err := store.CreateUser(otherUser); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create note as first user
	note := fixtures.NoteForCompany(data.Company.ID,
		fixtures.WithNoteAuthorID(data.User.ID),
	)
	if err := store.CreateNote(note); err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	// Try to delete as different user (no error, but no effect)
	_ = store.DeleteNote(note.ID, fixtures.DefaultOwnerID, otherUser.ID)

	// Note should still exist
	loaded, err := store.GetNoteByID(note.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("Note should still exist: %v", err)
	}
	if loaded.ID != note.ID {
		t.Error("Note was deleted by wrong author")
	}
}

func TestNote_SearchFilter(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Create notes with different content
	notes := []struct {
		title string
		body  string
		tags  string
	}{
		{"Meeting Protokoll", "Besprechung am Montag", "meeting"},
		{"Telefonat", "Kunde hat angerufen", "telefon,support"},
		{"Vertrag", "Vertragsverlängerung besprochen", "vertrag,wichtig"},
	}

	for _, n := range notes {
		note := fixtures.NoteForCompany(data.Company.ID,
			fixtures.WithNoteTitle(n.title),
			fixtures.WithNoteBody(n.body),
			fixtures.WithNoteTags(n.tags),
		)
		if err := store.CreateNote(note); err != nil {
			t.Fatalf("CreateNote failed: %v", err)
		}
	}

	tests := []struct {
		search    string
		wantCount int
	}{
		{"Meeting", 1},
		{"Kunde", 1},
		{"wichtig", 1},
		{"Montag", 1},
		{"xyz", 0},
		{"", 3}, // empty search returns all
	}

	for _, tt := range tests {
		t.Run("search_"+tt.search, func(t *testing.T) {
			results, err := store.ListNotesForParent(
				fixtures.DefaultOwnerID,
				model.ParentTypeCompany,
				data.Company.ID,
				model.NoteFilters{Search: tt.search},
			)
			if err != nil {
				t.Fatalf("ListNotesForParent failed: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("Search %q: got %d results, want %d", tt.search, len(results), tt.wantCount)
			}
		})
	}
}

func TestNote_InvalidParentType(t *testing.T) {
	store := fixtures.NewTestStore(t)

	note := &model.Note{
		OwnerID:    fixtures.DefaultOwnerID,
		AuthorID:   fixtures.DefaultOwnerID,
		ParentType: "invalid",
		ParentID:   1,
		Title:      "Test",
		Body:       "Test",
	}

	err := store.CreateNote(note)
	if err == nil {
		t.Error("Expected error for invalid parent type, got nil")
	}
}
