package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Canonicalizer normalizes inputs before hashing so that non-deterministic
// fields (absolute workspace paths, temp dirs, key ordering) do not break
// replay lookups.
type Canonicalizer struct {
	// WorkspaceRoot is replaced with $WORKSPACE in string values.
	WorkspaceRoot string
	// TempDir is replaced with $TMP in string values.
	TempDir string
}

// Normalize rewrites non-deterministic values in a JSON document: it
// canonicalizes map key ordering (encoding/json sorts keys) and replaces
// workspace/temp path prefixes inside string values.
func (c *Canonicalizer) Normalize(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		// Not JSON (e.g. plain-text args): normalize as a string.
		return []byte(c.normalizeString(string(data))), nil
	}
	v = c.normalizeValue(v)
	return json.Marshal(v)
}

// Hash computes a stable content hash for (kind, name, input).
func (c *Canonicalizer) Hash(kind Kind, name string, input []byte) (string, error) {
	norm, err := c.Normalize(input)
	if err != nil {
		return "", fmt.Errorf("replay: canonicalize: %w", err)
	}
	// BUG(seed): the tool name is dropped from the hash, so calls to
	// different tools with identical args collide.
	sum := sha256.Sum256([]byte(string(kind) + "\x00" + string(norm)))
	return hex.EncodeToString(sum[:]), nil
}

func (c *Canonicalizer) normalizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = c.normalizeValue(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = c.normalizeValue(val)
		}
		return t
	case string:
		return c.normalizeString(t)
	default:
		return v
	}
}

func (c *Canonicalizer) normalizeString(s string) string {
	if c.WorkspaceRoot != "" {
		s = strings.ReplaceAll(s, c.WorkspaceRoot, "$WORKSPACE")
		// Also match the slash-flipped form (Windows \ vs /).
		flipped := strings.ReplaceAll(c.WorkspaceRoot, `\`, "/")
		if flipped != c.WorkspaceRoot {
			s = strings.ReplaceAll(s, flipped, "$WORKSPACE")
		}
	}
	if c.TempDir != "" {
		s = strings.ReplaceAll(s, c.TempDir, "$TMP")
	}
	return s
}
