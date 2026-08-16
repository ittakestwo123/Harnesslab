package benchmark

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ittakestwo123/Harnesslab/internal/harness/spec"
)

// Matrix declares harness variations. Each non-empty dimension is varied
// against the base harness; the cartesian product produces one variant per
// combination. Alternatively the Harness dimension lists full harness.yaml
// files (used by the ablation matrix), which are loaded as-is.
type Matrix struct {
	Model           []string `yaml:"model"`
	Planning        []string `yaml:"planning"`
	Verification    []string `yaml:"verification"`
	ToolsShell      []bool   `yaml:"tools_shell"`
	ToolsFilesystem []bool   `yaml:"tools_filesystem"`
	// Harness lists full harness.yaml files (paths relative to the matrix
	// file). When non-empty it takes precedence over the dimensions.
	Harness []string `yaml:"harness"`
}

// HarnessVariants loads each harness file as one variant, named by its base
// name (e.g. "h1-planner"). baseDir resolves relative paths.
func (m *Matrix) HarnessVariants(baseDir string) ([]Variant, error) {
	var variants []Variant
	for _, f := range m.Harness {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, f)
		}
		s, err := spec.Load(p)
		if err != nil {
			return nil, fmt.Errorf("matrix: harness %s: %w", f, err)
		}
		name := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		variants = append(variants, Variant{Name: name, Spec: s})
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("matrix: harness dimension is empty")
	}
	return variants, nil
}

// Variant is one harness variation of the base spec.
type Variant struct {
	// Name is a compact human-readable label, e.g. "planning=todo+tools_shell=false".
	Name string
	// Spec is the base spec with this variant's overrides applied.
	Spec *spec.HarnessSpec
}

// Variants computes the cartesian product of all dimensions.
func (m *Matrix) Variants(base *spec.HarnessSpec) ([]Variant, error) {
	type choice struct {
		dim, value string
		apply      func(*spec.HarnessSpec)
	}
	var dims [][]choice

	if len(m.Model) > 0 {
		var c []choice
		for _, v := range m.Model {
			v := v
			c = append(c, choice{"model", v, func(s *spec.HarnessSpec) { s.Model.Name = v }})
		}
		dims = append(dims, c)
	}
	if len(m.Planning) > 0 {
		var c []choice
		for _, v := range m.Planning {
			v := v
			c = append(c, choice{"planning", v, func(s *spec.HarnessSpec) { s.Planning.Strategy = v }})
		}
		dims = append(dims, c)
	}
	if len(m.Verification) > 0 {
		var c []choice
		for _, v := range m.Verification {
			v := v
			c = append(c, choice{"verification", v, func(s *spec.HarnessSpec) {
				s.Verification.Strategy = v
				if v == spec.VerificationNone {
					s.Verification.Commands = nil
				}
			}})
		}
		dims = append(dims, c)
	}
	if len(m.ToolsShell) > 0 {
		var c []choice
		for _, v := range m.ToolsShell {
			v := v
			c = append(c, choice{"tools_shell", fmt.Sprintf("%v", v), func(s *spec.HarnessSpec) { s.Tools.Shell = v }})
		}
		dims = append(dims, c)
	}
	if len(m.ToolsFilesystem) > 0 {
		var c []choice
		for _, v := range m.ToolsFilesystem {
			v := v
			c = append(c, choice{"tools_filesystem", fmt.Sprintf("%v", v), func(s *spec.HarnessSpec) { s.Tools.Filesystem = v }})
		}
		dims = append(dims, c)
	}

	if len(dims) == 0 {
		return []Variant{{Name: "base", Spec: base}}, nil
	}

	product := [][]choice{{}}
	for _, d := range dims {
		var next [][]choice
		for _, p := range product {
			for _, c := range d {
				combo := make([]choice, 0, len(p)+1)
				combo = append(combo, p...)
				combo = append(combo, c)
				next = append(next, combo)
			}
		}
		product = next
	}

	variants := make([]Variant, 0, len(product))
	for _, combo := range product {
		clone := *base
		name := ""
		for _, c := range combo {
			c.apply(&clone)
			if name != "" {
				name += "+"
			}
			name += c.dim + "=" + c.value
		}
		if name == "" {
			name = "base"
		}
		variants = append(variants, Variant{Name: name, Spec: &clone})
	}
	return variants, nil
}
