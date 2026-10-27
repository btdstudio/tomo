package icm

import "testing"

func TestCheckClean(t *testing.T) {
	if problems := Check(); len(problems) > 0 {
		t.Fatalf("scaffold problems: %v", problems)
	}
}
