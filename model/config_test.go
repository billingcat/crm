package model_test

import (
	"strings"
	"testing"

	"github.com/billingcat/crm/model"
)

func TestConfig_CheckCookieSecret(t *testing.T) {
	for _, tc := range []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"empty", "", true},
		{"example default", "some secret", true},
		{"one byte short", strings.Repeat("x", model.MinCookieSecretLen-1), true},
		{"minimum length", strings.Repeat("x", model.MinCookieSecretLen), false},
		{"long", strings.Repeat("x", 64), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &model.Config{CookieSecret: tc.secret}
			if err := cfg.CheckCookieSecret(); (err != nil) != tc.wantErr {
				t.Errorf("CheckCookieSecret() err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
