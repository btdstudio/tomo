// Package state configures kiso's phase machine for tomo and migrates workspaces
// that kept their state under .tomo/.
//
// The loop is requirements → design → plan → implement. Implement is a run phase;
// the rest are gated.
//
// Layout:
//
//	STATE.json         phase machine: { schema, changes } maps each change folder
//	                   to its own { phase, status, gates, sources, updatedAt }
//	                   record, committed so approvals survive a clone
//	LOG.md             one timestamped line per state change / gate (committed)
//	.tomo/active       the active change's name; gitignored, because each checkout
//	                   works on its own change
//	.tomo/SLIP.md      handoff context for compact/resume (gitignored)
//	01_planning/specs/ per-change folders: <id>-<slug>/requirements.md,
//	                   design.md, tasks.json, plus ad-hoc outputs folded into
//	                   the active change
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	kiso "github.com/btdstudio/kiso/state"
	"github.com/btdstudio/kiso/workspace"
)

// Machine is the SDD loop.
var Machine = kiso.Machine{Tool: "tomo", Phases: []kiso.Phase{
	{Name: "requirements", Kind: kiso.Gated},
	{Name: "design", Kind: kiso.Gated},
	{Name: "plan", Kind: kiso.Gated},
	{Name: "implement", Kind: kiso.Run},
}}

// DefaultArtifactDir is where per-change spec artifacts live, relative to the
// project root (the ICM scaffold's 01_planning/specs/ convention).
const DefaultArtifactDir = "01_planning/specs"

// Now is the injectable clock used for timestamps (updatedAt and log lines).
// Tests override it to make output deterministic.
var Now = time.Now

func store(artifactDir string) kiso.Store {
	if artifactDir == "" {
		artifactDir = DefaultArtifactDir
	}
	return kiso.Store{
		Machine:   Machine,
		SpecsDir:  filepath.FromSlash(artifactDir),
		Commit:    true,
		Now:       func() time.Time { return Now() },
		Preflight: refuseOldState,
	}
}

type (
	// State is one change's machine.
	State = kiso.Change
	// KnownChange is one change folder annotated with any recorded progress.
	KnownChange = kiso.KnownChange
)

func Workspace(root string) string  { return workspace.Dir(root, "tomo") }
func StatePath(root string) string  { return store("").StatePath(root) }
func ActivePath(root string) string { return workspace.ActivePath(root, "tomo") }
func LogPath(root string) string    { return store("").LogPath(root) }
func SlipPath(root string) string   { return filepath.Join(Workspace(root), "SLIP.md") }

func SpecsDir(root, artifactDir string) string { return store(artifactDir).SpecsPath(root) }

// ChangeDir returns the path to one change folder. An empty change resolves to
// the specs base itself (a guard for ad-hoc outputs when nothing is active).
func ChangeDir(root, change, artifactDir string) string {
	return store(artifactDir).ChangeDir(root, change)
}

// ArtifactPath is an artifact inside a specific change folder.
func ArtifactPath(root, change, name, artifactDir string) string {
	return filepath.Join(ChangeDir(root, change, artifactDir), name)
}

// NextChangeId is the next sequential change id.
func NextChangeId(root, artifactDir string) int { return store(artifactDir).NextID(root) }

// Slugify lowercases to a short kebab-case slug.
func Slugify(text string) string { return kiso.Slugify(text) }

// DefaultState is the machine when nothing is recorded.
func DefaultState() State { return kiso.NewChange("") }

// ReadActive returns the active change's name ("" when none is set).
func ReadActive(root string) (string, error) { return workspace.ReadActive(root, "tomo") }

// LoadState resolves the active machine.
func LoadState(root string) (State, error) { return store("").Load(root) }

// SaveState writes the change's record and makes it the active change.
func SaveState(root string, st *State) error { return store("").Save(root, st) }

// ReadChanges returns the recorded changes.
func ReadChanges(root string) (map[string]*State, error) { return store("").ReadChanges(root) }

// KnownChanges lists the numbered folders under the artifact dir, sorted by id.
func KnownChanges(root, artifactDir string) ([]KnownChange, error) {
	return store(artifactDir).Known(root)
}

// SwitchTo points the workspace at a different change folder.
func SwitchTo(root, name, artifactDir string) (State, error) {
	return store(artifactDir).Switch(root, name)
}

// SetGate starts the given phase and sets its status.
func SetGate(st *State, phase, status, reason string) error {
	return Machine.SetGate(st, phase, status, reason)
}

// CheckGate reports whether setting phase to status respects the loop.
func CheckGate(st State, phase, status string) error { return Machine.CheckGate(st, phase, status) }

// SeedChange creates a change folder and records its initial machine, making it
// the active change. Sources are committed with it, so a clone has what the record
// points at.
func SeedChange(root, name, artifactDir string, sources []string) (State, error) {
	return store(artifactDir).Seed(root, name, kiso.SeedOptions{Sources: sources, Paths: sources})
}

// AdvanceGate sets a gate on the active change, logs it with any notes, and
// commits.
func AdvanceGate(root, artifactDir, phase, status, reason string, notes ...string) (State, error) {
	return store(artifactDir).Advance(root, phase, status, reason, notes...)
}

// Reopen moves the active change back to a phase, logs it and commits.
func Reopen(root, artifactDir, phase, reason string) (State, error) {
	return store(artifactDir).Reopen(root, phase, reason)
}

// CloseChange closes a change by name, logs it and commits.
func CloseChange(root, artifactDir, name, reason string) (State, error) {
	return store(artifactDir).CloseChange(root, name, reason)
}

// AppendLog appends one timestamped line to LOG.md.
func AppendLog(root, line string) error { return store("").Log(root, line) }

func oldStatePath(root string) string { return filepath.Join(Workspace(root), "state.json") }
func oldLogPath(root string) string   { return filepath.Join(Workspace(root), "log.md") }

func refuseOldLayout(root string) error {
	old := filepath.Join(Workspace(root), "specs")
	if _, err := os.Stat(old); err == nil {
		return fmt.Errorf("tomo: %s is from an earlier layout that this version doesn't support — run `tomo init` in a new workspace", old)
	}
	return nil
}

// refuseOldState stops a workspace with state under .tomo/ from silently looking
// empty.
func refuseOldState(root string) error {
	if err := refuseOldLayout(root); err != nil {
		return err
	}
	if _, err := os.Stat(oldStatePath(root)); err == nil {
		return fmt.Errorf("tomo: found old %s — run `tomo update` to move it to STATE.json", oldStatePath(root))
	}
	return nil
}

// MigrateOldState moves .tomo/state.json and .tomo/log.md to a committed
// STATE.json and LOG.md at the workspace root, and the active change to
// .tomo/active. It returns whether there was anything to move. Older files that
// keep the active machine at the top level are carried over too.
func MigrateOldState(root string) (bool, error) {
	if err := refuseOldLayout(root); err != nil {
		return false, err
	}
	data, err := os.ReadFile(oldStatePath(root))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("tomo: cannot read %s: %v", oldStatePath(root), err)
	}
	var old struct {
		kiso.Change
		Active  string                  `json:"change"`
		Changes map[string]*kiso.Change `json:"changes"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return false, fmt.Errorf("tomo: cannot parse %s: %v", oldStatePath(root), err)
	}
	if err := refuseRecordedState(root); err != nil {
		return false, err
	}
	changes := map[string]*kiso.Change{}
	for name, c := range old.Changes {
		if name != "" && c != nil {
			changes[name] = c
		}
	}
	if old.Active != "" && changes[old.Active] == nil {
		top := old.Change
		changes[old.Active] = &top
	}
	for _, c := range changes {
		if c.UpdatedAt == "" {
			c.UpdatedAt = old.UpdatedAt
		}
		for phase, status := range c.Gates {
			if status == "" {
				delete(c.Gates, phase)
			}
		}
		if c.Gates == nil {
			c.Gates = map[string]string{}
		}
		if c.Sources == nil {
			c.Sources = []string{}
		}
	}
	buf, err := json.MarshalIndent(map[string]any{"schema": kiso.Schema, "changes": changes}, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(StatePath(root), append(buf, '\n'), 0o644); err != nil {
		return false, err
	}
	if err := moveLog(root); err != nil {
		return false, err
	}
	if old.Active != "" {
		if err := workspace.WriteActive(root, "tomo", old.Active); err != nil {
			return false, err
		}
	}
	if err := os.Remove(oldStatePath(root)); err != nil {
		return false, err
	}
	return true, AppendLog(root, "migrated from .tomo/state.json")
}

// refuseRecordedState protects a STATE.json that already holds changes from being
// replaced by a migration.
func refuseRecordedState(root string) error {
	data, err := os.ReadFile(StatePath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var current struct {
		Changes map[string]json.RawMessage `json:"changes"`
	}
	if err := json.Unmarshal(data, &current); err != nil {
		return fmt.Errorf("tomo: cannot parse %s: %v", StatePath(root), err)
	}
	if len(current.Changes) > 0 {
		return fmt.Errorf("tomo: both %s and %s hold changes — keep one and remove the other", StatePath(root), oldStatePath(root))
	}
	return nil
}

// moveLog appends the old log to LOG.md, then removes it.
func moveLog(root string) error {
	data, err := os.ReadFile(oldLogPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	f, err := os.OpenFile(LogPath(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(oldLogPath(root))
}
