package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileEntry records a file's state at a point in time.
type FileEntry struct {
	Size    int64 `json:"size"`
	ModTime int64 `json:"mod_time_ns"`
}

// Manifest is a snapshot of all files in a project directory.
type Manifest map[string]FileEntry

// FileChange describes a file that was modified, added, or removed.
type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"` // modified, added, removed
}

// ValidationResult reports role boundary compliance.
type ValidationResult struct {
	Authorized []string     `json:"authorized"`
	Violations []FileChange `json:"violations"`
	Skipped    bool         `json:"skipped"`
	Reason     string       `json:"reason,omitempty"`
}

// CreateManifest snapshots all files in a project directory.
func CreateManifest(projectRoot string) Manifest {
	manifest := make(Manifest)
	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, path)

		// Skip directories, .git, node_modules, and run directories
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(rel, filepath.Join("status", "role-runs")) ||
				strings.HasPrefix(rel, filepath.Join("status", "review-runs")) {
				return filepath.SkipDir
			}
			return nil
		}

		manifest[rel] = FileEntry{
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		}
		return nil
	})
	return manifest
}

// SaveManifest writes a manifest to the project's status directory.
func SaveManifest(projectRoot string, manifest Manifest) string {
	manifestPath := filepath.Join(projectRoot, "status", "pre-execution-manifest.json")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	data, _ := json.MarshalIndent(manifest, "", "  ")
	tmpPath := manifestPath + fmt.Sprintf(".tmp.%d", os.Getpid())
	os.WriteFile(tmpPath, append(data, '\n'), 0644)
	os.Rename(tmpPath, manifestPath)
	return manifestPath
}

// LoadManifest reads a previously saved manifest.
func LoadManifest(projectRoot string) Manifest {
	manifestPath := filepath.Join(projectRoot, "status", "pre-execution-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	return manifest
}

// ArchiveManifest moves the manifest to a timestamped archive name.
func ArchiveManifest(projectRoot string) {
	src := filepath.Join(projectRoot, "status", "pre-execution-manifest.json")
	if _, err := os.Stat(src); err != nil {
		return
	}
	ts := time.Now().Format("2006-01-02T15-04-05")
	dst := filepath.Join(projectRoot, "status", fmt.Sprintf("pre-execution-manifest-%s.json", ts))
	os.Rename(src, dst)
}

// DetectChanges compares pre and post manifests to find file changes.
func DetectChanges(projectRoot string, pre Manifest) (changed, added, removed []string) {
	post := CreateManifest(projectRoot)

	for file, postEntry := range post {
		preEntry, existed := pre[file]
		if !existed {
			added = append(added, file)
		} else if preEntry.Size != postEntry.Size || preEntry.ModTime != postEntry.ModTime {
			changed = append(changed, file)
		}
	}

	for file := range pre {
		if _, exists := post[file]; !exists {
			removed = append(removed, file)
		}
	}
	return
}

// ValidateRoleChanges checks if file changes comply with a role's edit boundaries.
func ValidateRoleChanges(projectRoot string, pre Manifest, mayEdit []string) ValidationResult {
	if pre == nil || len(mayEdit) == 0 {
		return ValidationResult{Skipped: true, Reason: "missing manifest or capabilities"}
	}

	changed, added, removed := DetectChanges(projectRoot, pre)
	allModified := append(changed, added...)

	var authorized []string
	var violations []FileChange

	for _, file := range allModified {
		if matchesAnyPattern(file, mayEdit) {
			authorized = append(authorized, file)
		} else {
			violations = append(violations, FileChange{Path: file, Action: "modified"})
		}
	}

	for _, file := range removed {
		if !matchesAnyPattern(file, mayEdit) {
			violations = append(violations, FileChange{Path: file, Action: "removed"})
		}
	}

	return ValidationResult{
		Authorized: authorized,
		Violations: violations,
	}
}

// RevertUnauthorizedFiles removes new unauthorized files.
func RevertUnauthorizedFiles(projectRoot string, violations []FileChange) (reverted, failed []string) {
	for _, v := range violations {
		if v.Action != "modified" {
			continue
		}
		fullPath := filepath.Join(projectRoot, v.Path)
		if err := os.Remove(fullPath); err != nil {
			failed = append(failed, v.Path)
		} else {
			reverted = append(reverted, v.Path)
		}
	}
	return
}

// matchesAnyPattern checks if a file path matches any of the given glob patterns.
func matchesAnyPattern(filePath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesEditPattern(filePath, pattern) {
			return true
		}
	}
	return false
}

// matchesEditPattern checks a file path against a single glob-like pattern.
func matchesEditPattern(filePath, pattern string) bool {
	// Strip trailing comments/descriptions from glob patterns.
	// Examples: "status/* implementation artifacts" → "status/*"
	//           "tasks/* during shaping" → "tasks/*"
	//           "docs/decision-log.md" → unchanged (exact path, no glob)
	cleanPattern := strings.TrimSpace(pattern)
	lastStar := strings.LastIndex(cleanPattern, "*")
	if lastStar >= 0 && lastStar < len(cleanPattern)-1 {
		// There's text after the last * that isn't just "/" — strip it
		afterStar := cleanPattern[lastStar+1:]
		if afterStar != "*" && afterStar != "/" {
			// Keep the * and any immediately following / or *
			end := lastStar + 1
			for end < len(cleanPattern) && (cleanPattern[end] == '*' || cleanPattern[end] == '/') {
				end++
			}
			cleanPattern = strings.TrimSpace(cleanPattern[:end])
		}
	}

	if strings.HasSuffix(cleanPattern, "/**") {
		prefix := cleanPattern[:len(cleanPattern)-3]
		return strings.HasPrefix(filePath, prefix+"/") || filePath == prefix
	}
	if strings.HasSuffix(cleanPattern, "/*") {
		prefix := cleanPattern[:len(cleanPattern)-2]
		rest := strings.TrimPrefix(filePath, prefix+"/")
		return rest != filePath && !strings.Contains(rest, "/")
	}
	return filePath == cleanPattern
}
