package replay

import "testing"

// TestCanonicalizerNameIncluded pins that the tool name is part of the hash,
// so calls to different tools with identical arguments do not collide.
func TestCanonicalizerNameIncluded(t *testing.T) {
	c := &Canonicalizer{}
	hA, err := c.Hash(KindTool, "shell", []byte(`{"cmd":"ls"}`))
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	hB, err := c.Hash(KindTool, "filesystem", []byte(`{"cmd":"ls"}`))
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hA == hB {
		t.Fatal("hashes for different tools with identical args must differ")
	}
}
