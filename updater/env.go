// Tiny .env-file editor. The updater needs to persist the new
// PROCESSGIT_VERSION across docker compose restarts, which means rewriting
// the deployment's .env file.
//
// Comments, blank lines, and other keys are preserved verbatim. Quoted
// values on the target key are normalized to unquoted on write (we only
// ever write simple alphanumeric/dotted values, e.g. "0.1.2").

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// SetEnvFileKey reads path, updates (or appends) the given key=value pair,
// and atomically writes the file back. Returns the previous value of the
// key (empty string if it wasn't set, with second-return false to
// disambiguate from a key that was set to empty).
//
// A key that appears more than once: the FIRST occurrence is updated;
// subsequent occurrences are commented out with a "# duplicate" marker.
func SetEnvFileKey(path, key, value string) (previousValue string, hadKey bool, err error) {
	if key == "" {
		return "", false, errors.New("key must be non-empty")
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	// On non-existent file we'll just create one with the single line.
	lines := []string{}
	if len(data) > 0 {
		lines = strings.Split(string(data), "\n")
	}
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(trimmed[:eq])
		if k != key {
			continue
		}
		v := strings.TrimSpace(trimmed[eq+1:])
		if found {
			// Duplicate — comment it out for safety.
			lines[i] = "# " + line + "  # duplicate of " + key + " key, commented out by updater"
			continue
		}
		previousValue = strings.Trim(v, `"'`)
		lines[i] = key + "=" + value
		found = true
		hadKey = true
	}
	if !found {
		// Append, ensuring trailing newline.
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, key+"="+value, "")
	}
	return previousValue, hadKey, atomicWriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

// GetEnvFileKey returns the current value of the given key.
// Returns "", false if the file doesn't exist or the key isn't set.
func GetEnvFileKey(path, key string) (string, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		if k == key {
			v := strings.TrimSpace(line[eq+1:])
			return strings.Trim(v, `"'`), true, nil
		}
	}
	return "", false, sc.Err()
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}
