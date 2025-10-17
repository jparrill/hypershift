package syncglobalpullsecret

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// clearPullSecretContent removes all the invisible characters from the pull secret
// including CR, LF, TAB, and other characters, but normalizes the JSON format
// by removing extra spaces around colons and commas.
func clearPullSecretContent(pullSecret []byte) []byte {
	if len(pullSecret) == 0 {
		return pullSecret
	}

	// First, remove all control characters (but keep normal spaces)
	cleaned := make([]byte, 0, len(pullSecret))
	for _, b := range pullSecret {
		// Remove CR (\r), LF (\n), TAB (\t), and other control characters but keep spaces
		if b == 32 || (b >= 33 && b <= 126) {
			// Keep space (32) and printable ASCII characters (33-126)
			cleaned = append(cleaned, b)
		} else if b > 126 {
			// Keep extended ASCII/UTF-8 characters
			cleaned = append(cleaned, b)
		}
		// Skip control characters (0-31 except space) and DEL (127)
	}

	// Now normalize the JSON by removing extra spaces around punctuation
	normalized := make([]byte, 0, len(cleaned))
	for i, b := range cleaned {
		if b == ' ' {
			// Skip spaces that are before : or ,
			if i+1 < len(cleaned) && (cleaned[i+1] == ':' || cleaned[i+1] == ',') {
				continue
			}
			// Skip spaces that are after : or ,
			if i > 0 && (cleaned[i-1] == ':' || cleaned[i-1] == ',') {
				continue
			}
		}
		normalized = append(normalized, b)
	}

	return normalized
}

// comparePullSecretBytes compares two pull secret byte arrays to see if they are equivalent
func comparePullSecretBytes(a, b []byte) bool {
	trimmedA := clearPullSecretContent(a)
	trimmedB := clearPullSecretContent(b)
	return string(trimmedA) == string(trimmedB)
}

// readPullSecretFromFile reads a pull secret from a mounted file path
func readPullSecretFromFile(filePath string) ([]byte, error) {
	content, err := readFileFunc(filePath)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func validateDockerConfigJSON(b []byte) error {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if _, ok := m["auths"]; !ok {
		return fmt.Errorf("missing 'auths' key")
	}
	return nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".config.json.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func restartSystemdUnit(conn dbusConn, unitName string) error {
	ch := make(chan string)
	if _, err := conn.RestartUnit(unitName, dbusRestartUnitMode, ch); err != nil {
		return fmt.Errorf("failed to restart %s: %w", unitName, err)
	}

	// Wait for the result of the restart
	result := <-ch
	if result != systemdJobDone {
		return fmt.Errorf("failed to restart %s, result: %s", unitName, result)
	}

	return nil
}
