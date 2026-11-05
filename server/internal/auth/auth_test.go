package auth

import "testing"

func TestNewCode(t *testing.T) {
	for range 5000 {
		code, err := NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidCode(code) {
			t.Fatalf("codice non valido: %q", code)
		}
		if last := code[len(code)-1]; last == 'z' || last == 'Z' {
			t.Fatalf("codice che finisce in z (riservato su Cloud Run): %q", code)
		}
	}
}
