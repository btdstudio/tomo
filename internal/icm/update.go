package icm

import (
	"fmt"
	"os"
	"path/filepath"
)

// UpdateReport describes what Update did.
type UpdateReport struct {
	Updated   []string
	Skipped   []string
	Unchanged []string
}

// Update refreshes managed files to the installed version. A file is
// overwritten only when its on-disk content still matches the recorded
// manifest hash; user-edited files are skipped and reported.
func Update(dir string) (UpdateReport, error) {
	rep := UpdateReport{}
	cur, err := ReadManifest(dir)
	if err != nil {
		return rep, fmt.Errorf("tomo update: %v", err)
	}
	next, err := BuildManifest()
	if err != nil {
		return rep, err
	}
	for rel, nextHash := range next.Files {
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		curData, err := os.ReadFile(dest)
		if err != nil {
			if err := writeScaffoldFile(dir, rel); err != nil {
				return rep, err
			}
			rep.Updated = append(rep.Updated, rel)
			continue
		}
		prevHash, recorded := cur.Files[rel]
		if recorded && hashBytes(curData) == prevHash {
			if prevHash == nextHash {
				rep.Unchanged = append(rep.Unchanged, rel)
			} else {
				if err := writeScaffoldFile(dir, rel); err != nil {
					return rep, err
				}
				rep.Updated = append(rep.Updated, rel)
			}
			continue
		}
		// unrecorded (new file) or user-edited — skip to avoid clobbering
		rep.Skipped = append(rep.Skipped, rel)
	}
	if err := WriteManifest(dir, next); err != nil {
		return rep, err
	}
	return rep, nil
}

func writeScaffoldFile(dir, rel string) error {
	data, err := readScaffoldFile(rel)
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
