package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/labstack/echo/v4"
)

func TestAPICheckCustomerNumber_ScopedToSessionOwner(t *testing.T) {
	store := fixtures.NewTestStore(t)
	for _, owner := range []uint{1, 2} {
		s := fixtures.Settings(fixtures.WithSettingsOwnerID(owner))
		s.CustomerNumberPrefix, s.CustomerNumberWidth = "K-", 3
		if err := store.SaveSettings(s); err != nil {
			t.Fatalf("SaveSettings(owner %d) failed: %v", owner, err)
		}
	}
	comp := fixtures.Company(fixtures.WithCompanyOwnerID(1), fixtures.WithCompanyCustomerNumber("K-001"))
	if err := store.SaveCompany(comp, 1, nil); err != nil {
		t.Fatalf("SaveCompany failed: %v", err)
	}

	ctrl := &controller{model: store}
	e := echo.New()

	for _, tc := range []struct {
		owner  uint
		wantOK bool
	}{
		{1, false}, // own number: taken
		{2, true},  // other tenant's number: not visible
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/customer-number/check?num=K-001", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("ownerid", tc.owner)

		if err := ctrl.apiCheckCustomerNumber(c); err != nil {
			t.Fatalf("owner %d: handler error: %v", tc.owner, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("owner %d: status = %d, want 200", tc.owner, rec.Code)
		}
		var resp struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("owner %d: decode response: %v", tc.owner, err)
		}
		if resp.OK != tc.wantOK {
			t.Errorf("owner %d: ok = %v, want %v", tc.owner, resp.OK, tc.wantOK)
		}
	}
}
