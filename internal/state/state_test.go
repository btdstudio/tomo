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
	if err := SetGate(&st, "requirements", "await-approval", ""); err != nil {
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
	if err := SetGate(&st, "design", "draft", ""); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "design" || st.Status != "draft" || st.Gates["design"] != "draft" {
		t.Fatalf("phase/status/gates.design = %q/%q/%q", st.Phase, st.Status, st.Gates["design"])
	}
	if st.Gates["requirements"] != "" {
		t.Fatalf("gates.requirements should stay empty, got %q", st.Gates["requirements"])
	}
}

func TestSetGateRejectsUnknownPhaseAndStatus(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "bogus", "draft", ""); err == nil || !strings.Contains(err.Error(), "unknown phase") {
		t.Fatalf("want unknown phase error, got %v", err)
	}
	if err := SetGate(&st, "plan", "nope", ""); err == nil || !strings.Contains(err.Error(), "unknown gate status") {
		t.Fatalf("want unknown gate status error, got %v", err)
	}
}

func TestImplementUsesRunPhaseStatuses(t *testing.T) {
	st := DefaultState()
	if err := SetGate(&st, "implement", "in-progress", ""); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "implement" || st.Status != "in-progress" || st.Gates["implement"] != "in-progress" {
		t.Fatalf("implement in-progress: %+v", st)
	}
	if err := SetGate(&st, "implement", "complete", ""); err != nil {
		t.Fatal(err)
	}
	if st.Status != "complete" || st.Gates["implement"] != "complete" {
		t.Fatalf("implement complete: %+v", st)
	}
	if err := SetGate(&st, "implement", "draft", ""); err == nil || !strings.Contains(err.Error(), "unknown gate status") {
		t.Fatalf("want unknown gate status (draft), got %v", err)
	}
	if err := SetGate(&st, "implement", "approved", ""); err == nil || !strings.Contains(err.Error(), "unknown gate status") {
		t.Fatalf("want unknown gate status (approved), got %v", err)
	}
}

func TestSaveStateWritesSlimShapeAndLoadResolves(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "1-alpha"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "2-beta"), 0o755)

	a := DefaultState()
	a.Name = "1-alpha"
	if err := SetGate(&a, "requirements", "approved", ""); err != nil {
		t.Fatal(err)
	}
	a.Sources = []string{"prd.md"}
	if err := SaveState(root, &a); err != nil {
		t.Fatal(err)
	}

	b, err := SwitchTo(root, "2-beta", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := SetGate(&b, "design", "draft", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &b); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	var sf struct {
		Schema  int               `json:"schema"`
		Changes map[string]*State `json:"changes"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		t.Fatal(err)
	}
	if sf.Schema != Schema {
		t.Fatalf("schema = %d", sf.Schema)
	}
	// The active change is local, not shared.
	if strings.Contains(string(raw), `"change"`) {
		t.Fatalf("STATE.json names an active change:\n%s", raw)
	}
	if name, err := ReadActive(root); err != nil || name != "2-beta" {
		t.Fatalf("active = %q, %v", name, err)
	}
	if sf.Changes["1-alpha"].Gates["requirements"] != "approved" {
		t.Fatalf("1-alpha requirements = %q", sf.Changes["1-alpha"].Gates["requirements"])
	}
	if sf.Changes["2-beta"].Gates["design"] != "draft" {
		t.Fatalf("2-beta design = %q", sf.Changes["2-beta"].Gates["design"])
	}

	active, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if active.Name != "2-beta" || active.Phase != "design" || active.Status != "draft" {
		t.Fatalf("active = %+v", active)
	}

	switched, err := SwitchTo(root, "1-alpha", "")
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

func TestArtifactPaths(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "01_planning", "specs", "1-local-issue-store", "requirements.md")
	if got := ArtifactPath(root, "1-local-issue-store", "requirements.md", ""); got != want {
		t.Fatalf("artifactPath = %q, want %q", got, want)
	}
	if got := ArtifactPath(root, "", "findings.md", ""); got != filepath.Join(root, "01_planning", "specs", "findings.md") {
		t.Fatalf("artifactPath empty change = %q", got)
	}
}

func TestNextChangeId(t *testing.T) {
	root := t.TempDir()
	if NextChangeId(root, "") != 1 {
		t.Fatalf("no specs dir -> 1, got %d", NextChangeId(root, ""))
	}
	os.MkdirAll(SpecsDir(root, ""), 0o755)
	if NextChangeId(root, "") != 1 {
		t.Fatalf("empty specs -> 1")
	}
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "1-wrappers"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "2-tasks-layout"), 0o755)
	os.WriteFile(filepath.Join(SpecsDir(root, ""), "README.md"), []byte("x"), 0o644)
	if NextChangeId(root, "") != 3 {
		t.Fatalf("got %d, want 3", NextChangeId(root, ""))
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
	if ChangeDir(root, "", "") != SpecsDir(root, "") {
		t.Fatalf("empty change should resolve to specs base")
	}
	if ChangeDir(root, "3-thing", "") != filepath.Join(SpecsDir(root, ""), "3-thing") {
		t.Fatalf("named change path wrong")
	}
}

func TestEachChangeKeepsOwnMachine(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "1-local-issue-store"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "2-wrappers"), 0o755)

	st := DefaultState()
	st.Name = "1-local-issue-store"
	if err := SetGate(&st, "plan", "approved", ""); err != nil {
		t.Fatal(err)
	}
	st.Sources = []string{"inline statement"}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}

	other, err := SwitchTo(root, "2-wrappers", "")
	if err != nil {
		t.Fatal(err)
	}
	if other.Name != "2-wrappers" || other.Phase != "" || other.Status != "" {
		t.Fatalf("fresh folder = %+v", other)
	}

	back, err := SwitchTo(root, "1-local-issue-store", "")
	if err != nil {
		t.Fatal(err)
	}
	if back.Name != "1-local-issue-store" || back.Phase != "plan" || back.Status != "approved" {
		t.Fatalf("back = %+v", back)
	}
	if !reflect.DeepEqual(back.Sources, []string{"inline statement"}) {
		t.Fatalf("back sources = %v", back.Sources)
	}
	if back.Gates["plan"] != "approved" {
		t.Fatalf("back gates.plan = %q", back.Gates["plan"])
	}
}

func TestSwitchRejectsUnknownsAndKnownChangesLists(t *testing.T) {
	root := t.TempDir()
	if _, err := SwitchTo(root, "nope", ""); err == nil || !strings.Contains(err.Error(), "unknown change") {
		t.Fatalf("want unknown change (nope), got %v", err)
	}
	if _, err := SwitchTo(root, "", ""); err == nil || !strings.Contains(err.Error(), "unknown change") {
		t.Fatalf("want unknown change (empty), got %v", err)
	}
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "1-a"), 0o755)
	os.MkdirAll(filepath.Join(SpecsDir(root, ""), "2-b"), 0o755)
	os.WriteFile(filepath.Join(SpecsDir(root, ""), "README.md"), []byte("x"), 0o644)

	kc, err := KnownChanges(root, "")
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
	st.Name = "1-a"
	if err := SetGate(&st, "requirements", "await-approval", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(root, &st); err != nil {
		t.Fatal(err)
	}

	kc, _ = KnownChanges(root, "")
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
	b, _ := os.ReadFile(LogPath(root))
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
	st.Name = "1-x"
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
		if _, err := SwitchTo(root, name, ""); err == nil || !strings.Contains(err.Error(), "unknown change") {
			t.Fatalf("switch(%q) should be rejected, got %v", name, err)
		}
	}
}

func TestSwitchToHonorsCustomArtifactDir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "custom", "specs", "1-x"), 0o755)
	st, err := SwitchTo(root, "1-x", "custom/specs")
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "1-x" {
		t.Fatalf("change = %q", st.Name)
	}
}

func TestSpecsDirDefaultsAndCustom(t *testing.T) {
	root := t.TempDir()
	if got := SpecsDir(root, ""); got != filepath.Join(root, "01_planning", "specs") {
		t.Fatalf("default specs dir = %q", got)
	}
	if got := SpecsDir(root, "custom/specs"); got != filepath.Join(root, "custom", "specs") {
		t.Fatalf("custom specs dir = %q", got)
	}
}

func TestPhases(t *testing.T) {
	var names []string
	for _, p := range Machine.Phases {
		names = append(names, p.Name)
	}
	if !reflect.DeepEqual(names, []string{"intake", "requirements", "design", "plan", "implement", "test", "review", "security", "ship"}) {
		t.Fatalf("phases = %v", names)
	}
}

func TestCheckGateRequiresImplementComplete(t *testing.T) {
	st := State{Gates: map[string]string{"intake": "approved", "requirements": "approved", "design": "draft"}}
	if err := CheckGate(st, "plan", "draft"); err == nil || !strings.Contains(err.Error(), "requires design") {
		t.Fatalf("err = %v", err)
	}
	st.Gates["design"] = "approved"
	if err := CheckGate(st, "plan", "draft"); err != nil {
		t.Fatal(err)
	}
}

func writeOld(t *testing.T, root, body, log string) {
	t.Helper()
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldStatePath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldLogPath(root), []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOldStateIsRefusedUntilMigrated(t *testing.T) {
	root := t.TempDir()
	writeOld(t, root, `{"change":"","updatedAt":"","changes":{}}`, "")
	if _, err := LoadState(root); err == nil || !strings.Contains(err.Error(), "tomo update") {
		t.Fatalf("err = %v", err)
	}
}

func TestMigrateMovesStateLogAndActive(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(ChangeDir(root, "1-alpha", ""), 0o755)
	writeOld(t, root, `{
  "change": "1-alpha",
  "updatedAt": "2026-10-24T00:00:00.000Z",
  "changes": {
    "1-alpha": {"phase":"plan","status":"approved","gates":{"requirements":"approved","design":"approved","plan":"approved","implement":""},"sources":["00_issues/1.md"]},
    "2-beta": {"phase":"design","status":"draft","gates":{"requirements":"approved","design":"draft","plan":"","implement":""},"sources":[]},
    "": {"phase":"","status":"","gates":{},"sources":[]}
  }
}`, "- 2026-10-24T00:00:00.000Z old line\n")
	migrated, err := MigrateOldState(root)
	if err != nil || !migrated {
		t.Fatalf("migrated = %v, err = %v", migrated, err)
	}
	st, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"intake": "approved", "requirements": "approved", "design": "approved", "plan": "approved"}
	if st.Name != "1-alpha" || st.Status != "approved" || !reflect.DeepEqual(st.Gates, want) {
		t.Fatalf("st = %+v", st)
	}
	if st.UpdatedAt != "2026-10-24T00:00:00.000Z" || !reflect.DeepEqual(st.Sources, []string{"00_issues/1.md"}) {
		t.Fatalf("st = %+v", st)
	}
	changes, err := ReadChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes["2-beta"].Gates["design"] != "draft" {
		t.Fatalf("changes = %+v", changes)
	}
	log, _ := os.ReadFile(LogPath(root))
	if !strings.Contains(string(log), "old line") || !strings.Contains(string(log), "migrated from") {
		t.Fatalf("log = %q", log)
	}
	for _, p := range []string{oldStatePath(root), oldLogPath(root)} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s not removed: %v", p, err)
		}
	}
	if again, err := MigrateOldState(root); err != nil || again {
		t.Fatalf("second migration = %v, %v", again, err)
	}
}

func TestMigrateCarriesTopLevelMachine(t *testing.T) {
	root := t.TempDir()
	writeOld(t, root, `{"phase":"plan","status":"approved","gates":{"requirements":"approved","design":"approved","plan":"approved"},"sources":["legacy"],"change":"1-legacy","updatedAt":"2026-10-01T00:00:00.000Z"}`, "")
	if _, err := MigrateOldState(root); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "1-legacy" || st.Phase != "plan" || st.Gates["plan"] != "approved" || !reflect.DeepEqual(st.Sources, []string{"legacy"}) {
		t.Fatalf("st = %+v", st)
	}
}

func TestMigrateReplacesEmptySeedButNotRecordedChanges(t *testing.T) {
	root := t.TempDir()
	writeOld(t, root, `{"change":"1-x","changes":{"1-x":{"phase":"design","status":"draft","gates":{"design":"draft"}}}}`, "")
	if err := os.WriteFile(StatePath(root), []byte(`{"schema":1,"changes":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateOldState(root); err != nil {
		t.Fatal(err)
	}
	recorded, _ := os.ReadFile(StatePath(root))
	writeOld(t, root, `{"change":"2-y","changes":{"2-y":{"phase":"design","status":"draft"}}}`, "")
	if _, err := MigrateOldState(root); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("err = %v", err)
	}
	if after, _ := os.ReadFile(StatePath(root)); string(after) != string(recorded) {
		t.Fatalf("STATE.json overwritten: %s", after)
	}
}

func TestMigrateReportsCorruptFileAndLeavesIt(t *testing.T) {
	root := t.TempDir()
	writeOld(t, root, `{`, "")
	if _, err := MigrateOldState(root); err == nil || !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(oldStatePath(root)); string(got) != "{" {
		t.Fatalf("old file changed: %q", got)
	}
}

func TestEarlierLayoutIsRefused(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(Workspace(root), "specs", "1-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(root); err == nil || !strings.Contains(err.Error(), "earlier layout") {
		t.Fatalf("LoadState err = %v", err)
	}
	if _, err := MigrateOldState(root); err == nil || !strings.Contains(err.Error(), "earlier layout") {
		t.Fatalf("MigrateOldState err = %v", err)
	}
}

func TestMigrateSchema1MarksIntakeApproved(t *testing.T) {
	root := t.TempDir()
	body := `{"schema":1,"changes":{
  "1-done":{"phase":"implement","status":"complete","gates":{"requirements":"approved","design":"approved","plan":"approved","implement":"complete"}},
  "2-fresh":{"phase":"","status":"","gates":{}}
}}`
	if err := os.WriteFile(StatePath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := ReadChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	done := *changes["1-done"]
	if done.Gates["intake"] != "approved" || done.Gates["test"] != "" {
		t.Fatalf("gates = %+v", done.Gates)
	}
	if len(changes["2-fresh"].Gates) != 0 {
		t.Fatalf("fresh gates = %+v", changes["2-fresh"].Gates)
	}
	if err := CheckGate(done, "test", "draft"); err != nil {
		t.Fatalf("upgraded change should reach test: %v", err)
	}
	if after, _ := os.ReadFile(StatePath(root)); string(after) != body {
		t.Fatal("a read rewrote STATE.json")
	}
}

func TestRefusesNewerSchema(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(StatePath(root), []byte(`{"schema":99,"changes":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadChanges(root); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("err = %v", err)
	}
}
