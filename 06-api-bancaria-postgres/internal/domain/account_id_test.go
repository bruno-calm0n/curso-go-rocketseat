package domain

import "testing"

func TestParseAccountID(t *testing.T) {
	// O id publico combina tipo da conta e id numerico do banco.
	ref, err := ParseAccountID("pf_10")
	if err != nil {
		t.Fatalf("expected valid account id: %v", err)
	}

	if ref.Type != AccountTypePF || ref.ID != 10 {
		t.Fatalf("expected pf_10, got %#v", ref)
	}
}

func TestParseAccountIDErrors(t *testing.T) {
	tests := []string{"", "10", "pf", "pf_0", "xx_1", "pf_abc"}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			if _, err := ParseAccountID(test); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestFormatAccountID(t *testing.T) {
	if got := FormatAccountID(AccountTypePJ, 7); got != "pj_7" {
		t.Fatalf("expected pj_7, got %s", got)
	}
}
