// Package state owns the .tomo/ workspace contract: the state.json phase
// machine, log.md changeset appends, SLIP.md handoff, and the specs dir.
//
// It is a faithful port of src/lib/state.js. Command bodies reference this
// contract (paths, schema, transitions) via the `tomo` binary so the SDD loop
// behaves identically in every harness. Tests pin this exact behavior.
//
// Layout:
//
//	.tomo/state.json   phase machine: { change, updatedAt, changes } where
//	                   changes maps each change folder to its own
//	                   { phase, status, gates, sources } record — the single
//	                   store, so swapping specs preserves every change's
//	                   progress. The active machine is changes[change];
//	                   LoadState resolves it into { phase, status, gates,
//	                   sources, change, updatedAt }. Legacy files that still
//	                   carry the machine at top level keep loading.
//	.tomo/specs/       per-change folders: <id>-<slug>/requirements.md,
//	                   design.md, tasks.json, plus ad-hoc outputs folded into
//	                   the active change
//	.tomo/log.md       one timestamped line per state change / gate
//	.tomo/SLIP.md      handoff context for compact/resume
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The gated SDD phases in order, then the run phases. Later phases
// (documentation) may be added without breaking the schema.
var PHASES = []string{"requirements", "design", "plan", "implement"}

// Per-phase gate status. "" = not started. Gated phases only.
var GATE_STATES = []string{"", "draft", "await-approval", "approved"}

// Run-phase status for `implement` (driven by the implement loop, not a gate).
// "" = not started, "in-progress" = loop running or resumable, "complete" = done.
var IMPLEMENT_STATES = []string{"", "in-progress", "complete"}

// Now is the injectable clock used for timestamps (updatedAt and log lines).
// Tests override it to make output deterministic.
var Now = time.Now

// isoMillis renders a time exactly as JavaScript's Date.prototype.toISOString
// does: UTC with three-digit millisecond precision and a trailing 'Z'.
func isoMillis(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Gates is the per-phase gate map. Field order mirrors the JS insertion order
// (PHASES order), so marshaling preserves byte-identical JSON.
type Gates struct {
	Requirements string `json:"requirements"`
	Design       string `json:"design"`
	Plan         string `json:"plan"`
	Implement    string `json:"implement"`
}

// State is the resolved active machine (what LoadState returns).
type State struct {
	Phase     string
	Status    string
	Gates     Gates
	Sources   []string
	Change    string
	UpdatedAt string
}

// changeRecord is the per-change phase-machine record stored in state.json.
// Field order matches the JS record { phase, status, gates, sources }.
type changeRecord struct {
	Phase   string   `json:"phase"`
	Status  string   `json:"status"`
	Gates   Gates    `json:"gates"`
	Sources []string `json:"sources"`
}

// stateFile is the on-disk state.json shape read by LoadState/ReadChanges. It
// carries both the slim { change, updatedAt, changes } form and the legacy
// top-level machine fields, so either shape unmarshals cleanly.
type stateFile struct {
	Change    string                   `json:"change"`
	UpdatedAt string                   `json:"updatedAt"`
	Changes   map[string]*changeRecord `json:"changes"`

	// Legacy top-level machine (pre-`changes` files).
	Phase   string   `json:"phase"`
	Status  string   `json:"status"`
	Gates   Gates    `json:"gates"`
	Sources []string `json:"sources"`
}

// stateFileOut is the slim shape SaveState writes: exactly
// { change, updatedAt, changes }.
type stateFileOut struct {
	Change    string                   `json:"change"`
	UpdatedAt string                   `json:"updatedAt"`
	Changes   map[string]*changeRecord `json:"changes"`
}

func Workspace(root string) string { return filepath.Join(root, ".tomo") }

func StatePath(root string) string { return filepath.Join(Workspace(root), "state.json") }

func SpecsDir(root string) string { return filepath.Join(Workspace(root), "specs") }

// ChangeDir returns the path to one change folder. An empty change resolves to
// the specs base itself (a guard for ad-hoc outputs when nothing is active).
func ChangeDir(root, change string) string {
	if change != "" {
		return filepath.Join(SpecsDir(root), change)
	}
	return SpecsDir(root)
}

func LogPath(root string) string { return filepath.Join(Workspace(root), "log.md") }

func SlipPath(root string) string { return filepath.Join(Workspace(root), "SLIP.md") }

// ArtifactPath is an artifact inside a specific change folder.
func ArtifactPath(root, change, name string) string {
	return filepath.Join(ChangeDir(root, change), name)
}

var changeIDRe = regexp.MustCompile(`^(\d+)-`)

// changeNameRe is the strict grammar for an accepted change folder name: a
// numeric id, a hyphen, then the kebab-case slug Slugify produces. SwitchTo
// requires this so a name can never escape .tomo/specs via `..` or a separator.
var changeNameRe = regexp.MustCompile(`^(\d+)-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// NextChangeId is the next sequential change id: highest numeric `<id>-` prefix
// under specs + 1.
func NextChangeId(root string) int {
	entries, err := os.ReadDir(SpecsDir(root))
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		if m := changeIDRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases to a short kebab-case slug so change folders read as
// <id>-<slug>.
func Slugify(text string) string {
	s := strings.ToLower(text)
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	r := []rune(s)
	if len(r) > 40 {
		s = string(r[:40])
	}
	return s
}

func DefaultState() State {
	return State{
		Phase:   "",
		Status:  "",
		Gates:   Gates{},
		Sources: []string{},
		Change:  "",
	}
}

func normalizeSources(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// LoadState resolves the active machine. It returns DefaultState when no
// state.json exists, and errors when the file cannot be parsed.
func LoadState(root string) (State, error) {
	p := StatePath(root)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultState(), nil
		}
		return State{}, fmt.Errorf("tomo: cannot read %s: %v", p, err)
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return State{}, fmt.Errorf("tomo: cannot parse %s: %v", p, err)
	}

	st := DefaultState()
	// Legacy top-level machine fields (absent in the slim shape).
	st.Phase = sf.Phase
	st.Status = sf.Status
	st.Gates = sf.Gates
	st.Sources = normalizeSources(sf.Sources)
	st.Change = sf.Change
	st.UpdatedAt = sf.UpdatedAt

	// The active change's record under `changes` is the source of truth
	// (empty change resolves to the changes[""] record).
	if rec, ok := sf.Changes[sf.Change]; ok && rec != nil {
		if rec.Phase != "" {
			st.Phase = rec.Phase
		}
		if rec.Status != "" {
			st.Status = rec.Status
		}
		st.Gates = rec.Gates
		if rec.Sources != nil {
			st.Sources = append([]string{}, rec.Sources...)
		}
	}
	return st, nil
}

// SaveState stamps updatedAt and writes the slim { change, updatedAt, changes }
// shape, preserving every recorded change's record.
func SaveState(root string, st *State) error {
	st.UpdatedAt = isoMillis(Now())
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		return err
	}
	changes, err := ReadChanges(root)
	if err != nil {
		return err
	}
	changes[st.Change] = &changeRecord{
		Phase:   st.Phase,
		Status:  st.Status,
		Gates:   st.Gates,
		Sources: normalizeSources(st.Sources),
	}
	out := stateFileOut{
		Change:    st.Change,
		UpdatedAt: st.UpdatedAt,
		Changes:   changes,
	}
	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(StatePath(root), buf, 0o644)
}

// ReadChanges returns the per-change phase-machine records in state.json
// (empty when absent or unreadable).
func ReadChanges(root string) (map[string]*changeRecord, error) {
	data, err := os.ReadFile(StatePath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*changeRecord{}, nil
		}
		return map[string]*changeRecord{}, nil
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		// unreadable/missing map — treat as empty
		return map[string]*changeRecord{}, nil
	}
	if sf.Changes == nil {
		return map[string]*changeRecord{}, nil
	}
	return sf.Changes, nil
}

// KnownChange is one change folder annotated with any recorded progress.
type KnownChange struct {
	Name   string `json:"name"`
	ID     int    `json:"id"`
	Phase  string `json:"phase"`
	Status string `json:"status"`
	Gates  Gates  `json:"gates"`
}

// KnownChanges lists the numbered folders under .tomo/specs/ sorted by id.
func KnownChanges(root string) ([]KnownChange, error) {
	changes, _ := ReadChanges(root)
	out := []KnownChange{}
	entries, err := os.ReadDir(SpecsDir(root))
	if err != nil {
		return out, nil
	}
	for _, e := range entries {
		name := e.Name()
		m := changeIDRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		rec := changes[name]
		kc := KnownChange{Name: name, ID: id, Gates: Gates{}}
		if rec != nil {
			kc.Phase = rec.Phase
			kc.Status = rec.Status
			kc.Gates = rec.Gates
		}
		out = append(out, kc)
	}
	// os.ReadDir already returns names sorted; the numeric id sort is a no-op
	// for well-formed <id>- folders but keep it explicit for safety.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].ID > out[j].ID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out, nil
}

// SwitchTo points the workspace at a different change folder and resumes its
// phase machine. The target folder must exist under .tomo/specs/ and match the
// strict change-name grammar (no `..`, no separators, no symlinks).
func SwitchTo(root, name string) (State, error) {
	if !changeNameRe.MatchString(name) {
		return State{}, fmt.Errorf(`tomo: unknown change "%s" — run /tomo switch to list specs`, name)
	}
	dir := ChangeDir(root, name)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return State{}, fmt.Errorf(`tomo: unknown change "%s" — run /tomo switch to list specs`, name)
	}
	st := DefaultState()
	st.Change = name
	changes, _ := ReadChanges(root)
	if rec := changes[name]; rec != nil {
		if rec.Phase != "" {
			st.Phase = rec.Phase
		}
		if rec.Status != "" {
			st.Status = rec.Status
		}
		st.Gates = rec.Gates
		if rec.Sources != nil {
			st.Sources = append([]string{}, rec.Sources...)
		}
	}
	if err := SaveState(root, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

// SetGate advances the phase machine: starts the given phase and sets its
// status. Implement is a run phase and uses IMPLEMENT_STATES; gated phases use
// GATE_STATES. Does not enforce order (the gates do), but validates names.
func SetGate(st *State, phase, status string) error {
	found := false
	for _, p := range PHASES {
		if p == phase {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown phase: %s", phase)
	}
	var allowed []string
	label := "gate status"
	if phase == "implement" {
		allowed = IMPLEMENT_STATES
		label = "implement status"
	} else {
		allowed = GATE_STATES
	}
	for _, s := range allowed {
		if s == status {
			st.Phase = phase
			st.Status = status
			switch phase {
			case "requirements":
				st.Gates.Requirements = status
			case "design":
				st.Gates.Design = status
			case "plan":
				st.Gates.Plan = status
			case "implement":
				st.Gates.Implement = status
			}
			return nil
		}
	}
	return fmt.Errorf("unknown %s: %s", label, status)
}

// AppendLog appends one timestamped line to .tomo/log.md.
func AppendLog(root, line string) error {
	if err := os.MkdirAll(Workspace(root), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(LogPath(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- %s %s\n", isoMillis(Now()), line)
	return err
}
