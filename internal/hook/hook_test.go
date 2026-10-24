package hook

import (
	"testing"

	"github.com/btdstudio/tomo/internal/state"
)

func TestSessionMessageSilentWithoutWorkspace(t *testing.T) {
	if msg := SessionMessage(t.TempDir()); msg != "" {
		t.Fatalf("expected empty, got %q", msg)
	}
}

func TestSessionMessageReportsNextGate(t *testing.T) {
	root := t.TempDir()
	st := state.DefaultState()
	st.Change = "1-x"
	if err := state.SetGate(&st, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	msg := SessionMessage(root)
	want := "Tomo: requirements (await-approval) — next gate: requirements (await-approval)"
	if msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}
}

func TestSessionMessageReportsComplete(t *testing.T) {
	root := t.TempDir()
	st := state.DefaultState()
	st.Change = "1-x"
	for _, g := range []struct{ p, s string }{
		{"requirements", "approved"}, {"design", "approved"}, {"plan", "approved"}, {"implement", "complete"},
	} {
		if err := state.SetGate(&st, g.p, g.s); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	msg := SessionMessage(root)
	if msg != "Tomo: implement (complete) — spec complete — /tomo commit or /tomo switch" {
		t.Fatalf("got %q", msg)
	}
}

func TestSessionMessageReportsAllApproved(t *testing.T) {
	root := t.TempDir()
	st := state.DefaultState()
	st.Change = "1-x"
	for _, g := range []struct{ p, s string }{
		{"requirements", "approved"}, {"design", "approved"}, {"plan", "approved"},
	} {
		if err := state.SetGate(&st, g.p, g.s); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	msg := SessionMessage(root)
	if msg != "Tomo: plan (approved) — all gates approved — /tomo implement to run the plan" {
		t.Fatalf("got %q", msg)
	}
}
