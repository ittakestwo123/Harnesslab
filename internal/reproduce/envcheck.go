package reproduce

import "encoding/json"

// EnvMode controls how environment drift is treated during reproduction.
type EnvMode string

const (
	// EnvWarn prints drift but continues (default).
	EnvWarn EnvMode = "warn"
	// EnvStrict fails the reproduction on any drift.
	EnvStrict EnvMode = "strict"
	// EnvIgnore skips validation.
	EnvIgnore EnvMode = "ignore"
)

// EnvDiff describes one environment dimension that changed.
type EnvDiff struct {
	Key      string `json:"key"`
	Recorded string `json:"recorded"`
	Current  string `json:"current"`
	Match    bool   `json:"match"`
}

// ParseEnvMode maps a string to an EnvMode (default warn).
func ParseEnvMode(s string) EnvMode {
	switch s {
	case "strict":
		return EnvStrict
	case "ignore":
		return EnvIgnore
	default:
		return EnvWarn
	}
}

// CompareEnv compares a recorded environment against the current one.
// Dimensions unknown on either side are skipped.
func CompareEnv(recorded, current Environment) []EnvDiff {
	items := []struct {
		key  string
		a, b string
	}{
		{"OS", recorded.GOOS, current.GOOS},
		{"Arch", recorded.GOARCH, current.GOARCH},
		{"Go", recorded.GoVersion, current.GoVersion},
		{"Git", recorded.GitVersion, current.GitVersion},
		{"HarnessLab", recorded.HarnessLab, current.HarnessLab},
		{"TRPCAgentGo", recorded.TRPCAgentGo, current.TRPCAgentGo},
	}
	var diffs []EnvDiff
	for _, it := range items {
		if it.a == "" || it.b == "" {
			continue
		}
		diffs = append(diffs, EnvDiff{Key: it.key, Recorded: it.a, Current: it.b, Match: it.a == it.b})
	}
	return diffs
}

// EnvFromJSON decodes an Environment from JSON.
func EnvFromJSON(data string) (Environment, error) {
	var env Environment
	if data == "" {
		return env, nil
	}
	if err := json.Unmarshal([]byte(data), &env); err != nil {
		return env, err
	}
	return env, nil
}
