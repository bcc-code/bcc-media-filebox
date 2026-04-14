package config

import (
	"fmt"
	"os"
)

type Target struct {
	Name string
	Dir  string
}

// LoadTargets reads numbered TARGET_N_NAME / TARGET_N_DIR environment variable
// pairs (starting at 1) and returns the configured targets. It returns an error
// if no targets are configured, if a pair is incomplete, or if a directory does
// not exist.
func LoadTargets() ([]Target, error) {
	var targets []Target

	for i := 1; ; i++ {
		name := os.Getenv(fmt.Sprintf("TARGET_%d_NAME", i))
		dir := os.Getenv(fmt.Sprintf("TARGET_%d_DIR", i))

		if name == "" && dir == "" {
			break
		}
		if name == "" {
			return nil, fmt.Errorf("TARGET_%d_DIR is set but TARGET_%d_NAME is missing", i, i)
		}
		if dir == "" {
			return nil, fmt.Errorf("TARGET_%d_NAME is set but TARGET_%d_DIR is missing", i, i)
		}

		info, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("target %q directory %q: %w", name, dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("target %q: %q is not a directory", name, dir)
		}

		targets = append(targets, Target{Name: name, Dir: dir})
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets configured (set TARGET_1_NAME and TARGET_1_DIR environment variables)")
	}

	return targets, nil
}

// TargetDir returns the directory for the given target name, or empty string if not found.
func TargetDir(targets []Target, name string) (string, bool) {
	for _, t := range targets {
		if t.Name == name {
			return t.Dir, true
		}
	}
	return "", false
}
