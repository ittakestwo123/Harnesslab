package replay

import (
	"strings"
	"testing"
)

func TestNormalizeNestedJSONPath(t *testing.T) {
	c := &Canonicalizer{WorkspaceRoot: `C:\ws\worktrees\run-abc`}
	// A model request whose tool message content embeds the tool result JSON
	// (with the workspace path) as a string value — the shape that broke
	// offline replay when a tool result followed the first model call.
	input := []byte(`{"messages":[{"role":"tool","content":"{\"base_directory\":\"C:\\ws\\worktrees\\run-abc\",\"file_name\":\"README.md\",\"contents\":\"hi\"}"}],"tools":[]}`)
	norm, err := c.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	s := string(norm)
	if !strings.Contains(s, "$WORKSPACE") {
		t.Fatalf("nested workspace path not normalized: %s", s)
	}
	if strings.Contains(s, "run-abc") {
		t.Fatalf("nested path leaked: %s", s)
	}
}

func TestNormalizeKeepsPlainStrings(t *testing.T) {
	c := &Canonicalizer{WorkspaceRoot: `C:\ws`}
	norm, err := c.Normalize([]byte(`{"content":"just some text","n":"123","arr":["a b"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(norm), "just some text") || !strings.Contains(string(norm), "123") {
		t.Fatalf("plain strings mangled: %s", norm)
	}
}
