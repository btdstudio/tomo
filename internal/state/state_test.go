package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

func writeJSON(t *testing.T, p string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStateReturnsDefaultWhenAbsent(t *testing.T) {
	st, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(st, DefaultState()) {
		t.Fatalf("got %+v, want default", st)
	}
}

func TestSaveThenLoadRoundtripsAndStampsUpdatedAt(t *testing.T) {
	root := t.TempDir()
	st := DefaultState()
	if err := SetGate(&st, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}
	st.Sources = []string{"specs/prd.md"}
	wantGates := st.Gates
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}
	if st.UpdatedAt == "" {
		t.Fatal("updatedAt must be stamped")
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != "requirements" || loaded.Status != "await-approval" {
		t.Fatalf("phase/status = %q/%q", loaded.Phase, loaded.Status)
	}
	if !reflect.DeepEqual(loaded.Sources, []string{"specs/prd.md"}) {
		t.Fatalf("sources = %v", loaded.Sources)
	}
	if !reflect.DeepEqual(loaded.Gates, wantGates) {
		t.Fatalf("gates = %+v, want %+v", loaded.Gates, wantGates)
	}
}

func TestSetGateUpdatesPhaseStatusAndMap(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "design", "draft"); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "design" || st.Status != "draft" || st.Gates.Design != "draft" {
		t.Fatalf("phase/status/gates.design = %q/%q/%q", st.Phase, st.Status, st.Gates.Design)
	}
	if st.Gates.Requirements != "" {
		t.Fatalf("gates.requirements should stay empty, got %q", st.Gates.Requirements)
	}
}

func TestSetGateRejectsUnknownPhaseAndStatus(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "bogus", "draft"); err == nil || !strings.Contains(err.Error(), "unknown phase") {
		t.Fatalf("want unknown phase error, got %v", err)
	}
	if err := SetGate(&st, "plan", "nope"); err == nil || !strings.Contains(err.Error(), "unknown gate status") {
		t.Fatalf("want unknown gate status error, got %v", err)
	}
}

func TestDefaultStateSeedsEveryPhase(t *testing.T) {
	if !reflect.DeepEqual(PHASES, []string{"requirements", "design", "plan", "implement"}) {
		t.Fatalf("PHASES = %v", PHASES)
	}
	if !reflect.DeepEqual(IMPLEMENT_STATES, []string{"", "in-progress", "complete"}) {
		t.Fatalf("IMPLEMENT_STATES = %v", IMPLEMENT_STATES)
	}
	g := DefaultState().Gates
	if g.Requirements != "" || g.Design != "" || g.Plan != "" || g.Implement != "" {
		t.Fatalf("gates should be all empty, got %+v", g)
	}
}

func TestImplementUsesRunPhaseStatuses(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "implement", "in-progress"); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "implement" || st.Status != "in-progress" || st.Gates.Implement != "in-progress" {
		t.Fatalf("implement in-progress: %+v", st)
	}
	if err := SetGate(&st, "implement", "complete"); err != nil {
		t.Fatal(err)
	}
	if st.Status != "complete" || st.Gates.Implement != "complete" {
		t.Fatalf("implement complete: %+v", st)
	}
	if err := SetGate(&st, "implement", "draft"); err == nil || !strings.Contains(err.Error(), "unknown implement status") {
		t.Fatalf("want unknown implement status (draft), got %v", err)
	}
	if err := SetGate(&st, "implement", "approved"); err == nil || !strings.Contains(err.Error(), "unknown implement status") {
		t.Fatalf("want unknown implement status (approved), got %v", err)
	}
}

func TestSaveStateWritesSlimShapeAndLoadResolves(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-alpha"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-beta"), 0o755)

	a := DefaultState()
	a.Change = "1-alpha"
	if err := SetGate(&a, "requirements", "approved"); err != nil {
		t.Fatal(err)
	}
	a.Sources = []string{"prd.md"}
	if err := SaveState(root, &a); err != nil {
		t.Fatal(err)
	}

	b, err := SwitchTo(root, "2-beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := SetGate(&b, "design", "draft"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &b); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	var sf stateFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatal(err)
	}
	// Top-level key order: change, updatedAt, changes.
	s := string(raw)
	ci, ui, chi := strings.Index(s, `"change"`), strings.Index(s, `"updatedAt"`), strings.Index(s, `"changes"`)
	if !(ci >= 0 && ci < ui && ui < chi) {
		t.Fatalf("top-level key order wrong: change=%d updatedAt=%d changes=%d\n%s", ci, ui, chi, s)
	}
	if sf.Change != "2-beta" {
		t.Fatalf("change = %q", sf.Change)
	}
	if sf.Changes["1-alpha"].Gates.Requirements != "approved" {
		t.Fatalf("1-alpha requirements = %q", sf.Changes["1-alpha"].Gates.Requirements)
	}
	if sf.Changes["2-beta"].Gates.Design != "draft" {
		t.Fatalf("2-beta design = %q", sf.Changes["2-beta"].Gates.Design)
	}

	active, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.Change != "2-beta" || active.Phase != "design" || active.Status != "draft" {
		t.Fatalf("active = %+v", active)
	}

	switched, err := SwitchTo(root, "1-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if switched.Phase != "requirements" || switched.Status != "approved" {
		t.Fatalf("switched phase/status = %q/%q", switched.Phase, switched.Status)
	}
	if !reflect.DeepEqual(switched.Sources, []string{"prd.md"}) {
		t.Fatalf("switched sources = %v", switched.Sources)
	}
}

func TestLegacyThreeGateStateStillLoads(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(SpecsDir(root), 0o755)
	writeJSON(t, StatePath(root), map[string]any{
		"phase":   "plan",
		"status":  "approved",
		"gates":   map[string]string{"requirements": "approved", "design": "approved", "plan": "approved"},
		"sources": []string{"legacy"},
		"change":  "1-legacy",
	})
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Gates.Implement != "" || loaded.Gates.Plan != "approved" {
		t.Fatalf("gates = %+v", loaded.Gates)
	}
}

func TestArtifactPaths(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, ".tomo", "specs", "1-local-issue-store", "requirements.md")
	if got := ArtifactPath(root, "1-local-issue-store", "requirements.md"); got != want {
		t.Fatalf("artifactPath = %q, want %q", got, want)
	}
	if got := ArtifactPath(root, "", "findings.md"); got != filepath.Join(root, ".tomo", "specs", "findings.md") {
		t.Fatalf("artifactPath empty change = %q", got)
	}
}

func TestNextChangeId(t *testing.T) {
	root := t.TempDir()
	if NextChangeId(root) != 1 {
		t.Fatalf("no specs dir -> 1, got %d", NextChangeId(root))
	}
	os.MkdirAll(SpecsDir(root), 0o755)
	if NextChangeId(root) != 1 {
		t.Fatalf("empty specs -> 1")
	}
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-wrappers"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-tasks-layout"), 0o755)
	os.WriteFile(filepath.Join(SpecsDir(root), "README.md"), []byte("x"), 0o644)
	if NextChangeId(root) != 3 {
		t.Fatalf("got %d, want 3", NextChangeId(root))
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Local Issue Store!": "local-issue-store",
		"  ":                 "",
		"spec folder layout": "spec-folder-layout",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if len(Slugify("a "+strings.Repeat("x", 100))) > 40 {
		t.Fatalf("slugify too long")
	}
}

func TestChangeDirResolvesToSpecsBaseWhenEmpty(t *testing.T) {
	root := t.TempDir()
	if ChangeDir(root, "") != SpecsDir(root) {
		t.Fatalf("empty change should resolve to specs base")
	}
	if ChangeDir(root, "3-thing") != filepath.Join(SpecsDir(root), "3-thing") {
		t.Fatalf("named change path wrong")
	}
}

func TestEachChangeKeepsOwnMachine(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-local-issue-store"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-wrappers"), 0o755)

	st := DefaultState()
	st.Change = "1-local-issue-store"
	if err := SetGate(&st, "plan", "approved"); err != nil {
		t.Fatal(err)
	}
	st.Sources = []string{"inline statement"}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}

	other, err := SwitchTo(root, "2-wrappers")
	if err != nil {
		t.Fatal(err)
	}
	if other.Change != "2-wrappers" || other.Phase != "" || other.Status != "" {
		t.Fatalf("fresh folder = %+v", other)
	}

	back, err := SwitchTo(root, "1-local-issue-store")
	if err != nil {
		t.Fatal(err)
	}
	if back.Change != "1-local-issue-store" || back.Phase != "plan" || back.Status != "approved" {
		t.Fatalf("back = %+v", back)
	}
	if !reflect.DeepEqual(back.Sources, []string{"inline statement"}) {
		t.Fatalf("back sources = %v", back.Sources)
	}
	if back.Gates.Plan != "approved" {
		t.Fatalf("back gates.plan = %q", back.Gates.Plan)
	}
}

func TestSwitchRejectsUnknownsAndKnownChangesLists(t *testing.T) {
	root := t.TempDir()
	if _, err := SwitchTo(root, "nope"); err == nil || !strings.Contains(err.Error(), "unknown change") {
		t.Fatalf("want unknown change (nope), got %v", err)
	}
	if _, err := SwitchTo(root, ""); err == nil || !strings.Contains(err.Error(), "unknown change") {
		t.Fatalf("want unknown change (empty), got %v", err)
	}
	os.MkdirAll(filepath.Join(SpecsDir(root), "1-a"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root), "2-b"), 0o755)
	os.WriteFile(filepath.Join(SpecsDir(root), "README.md"), []byte("x"), 0o644)

	kc, err := KnownChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range kc {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"1-a", "2-b"}) {
		t.Fatalf("knownChanges names = %v", names)
	}

	st := DefaultState()
	st.Change = "1-a"
	if err := SetGate(&st, "requirements", "await-approval"); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}

	kc, _ = KnownChanges(root)
	find := func(name string) KnownChange {
		for _, c := range kc {
			if c.Name == name {
				return c
			}
		}
		return KnownChange{}
	}
	if find("1-a").Status != "await-approval" {
		t.Fatalf("1-a status = %q", find("1-a").Status)
	}
	if find("2-b").Status != "" {
		t.Fatalf("2-b status = %q", find("2-b").Status)
	}
}

func TestAppendLog(t *testing.T) {
	root := t.TempDir()
	if err := AppendLog(root, "requirements → await-approval"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(root, "requirements approved"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".tomo", "log.md"))
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "- 20") || !strings.Contains(lines[0], "T") {
		t.Fatalf("line 0 missing timestamp: %q", lines[0])
	}
	if !strings.Contains(lines[0], "await-approval") || !strings.Contains(lines[1], "approved") {
		t.Fatalf("lines wrong: %v", lines)
	}
}

func TestUpdatedAtFormatMatchesJSISOString(t *testing.T) {
	orig := Now
	defer func() { Now = orig }()
	Now = fixedNow(time.Date(2026, 10, 24, 9, 41, 59, 123000000, time.UTC))
	st := DefaultState()
	st.Change = "1-x"
	if err := SaveState(t.TempDir(), &st); err != nil {
		t.Fatal(err)
	}
	if st.UpdatedAt != "2026-10-24T09:41:59.123Z" {
		t.Fatalf("updatedAt = %q", st.UpdatedAt)
	}
}

func TestSwitchRejectsTraversalNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"1-../escape", "1-a/../../x", "1-Weird_Name", "nope", ""} {
		if _, err := SwitchTo(root, name); err == nil || !strings.Contains(err.Error(), "unknown change") {
			t.Fatalf("switch(%q) should be rejected, got %v", name, err)
		}
	}
}
