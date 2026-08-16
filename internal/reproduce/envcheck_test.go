package reproduce

import "testing"

func TestParseEnvMode(t *testing.T) {
	if ParseEnvMode("strict") != EnvStrict || ParseEnvMode("ignore") != EnvIgnore || ParseEnvMode("") != EnvWarn {
		t.Fatal("env mode parsing broken")
	}
}

func TestCompareEnv(t *testing.T) {
	recorded := Environment{GOOS: "windows", GOARCH: "amd64", GoVersion: "go1.26.5"}
	current := Environment{GOOS: "windows", GOARCH: "amd64", GoVersion: "go1.25.0", GitVersion: "git 2.45"}

	diffs := CompareEnv(recorded, current)
	if len(diffs) != 3 { // os, arch, go (git only recorded on one side -> skipped)
		t.Fatalf("diffs = %d, want 3: %+v", len(diffs), diffs)
	}
	byKey := map[string]EnvDiff{}
	for _, d := range diffs {
		byKey[d.Key] = d
	}
	if !byKey["OS"].Match || !byKey["Arch"].Match {
		t.Fatalf("os/arch should match: %+v", diffs)
	}
	if byKey["Go"].Match {
		t.Fatalf("go version should mismatch: %+v", diffs)
	}
}

func TestEnvFromJSON(t *testing.T) {
	env, err := EnvFromJSON(`{"goos":"windows","goarch":"amd64","go_version":"go1.26.5"}`)
	if err != nil || env.GOOS != "windows" || env.GoVersion == "" {
		t.Fatalf("EnvFromJSON = %+v, err=%v", env, err)
	}
	if _, err := EnvFromJSON(""); err != nil {
		t.Fatalf("empty env should be ok: %v", err)
	}
	if _, err := EnvFromJSON("{bad"); err == nil {
		t.Fatal("invalid json should error")
	}
}
