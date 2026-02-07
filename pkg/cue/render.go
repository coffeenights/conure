package cue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/modfile"
	"cuelang.org/go/mod/module"
)

// Renderer handles CUE module rendering from OCI registries.
type Renderer struct {
	ctx *cue.Context
	reg modconfig.Registry
}

// NewRenderer creates a new CUE renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		ctx: cuecontext.New(),
	}
}

// initRegistry lazily initializes the registry client.
func (r *Renderer) initRegistry() error {
	if r.reg != nil {
		return nil
	}
	reg, err := modconfig.NewRegistry(nil)
	if err != nil {
		return fmt.Errorf("failed to create registry: %w", err)
	}
	r.reg = reg
	return nil
}

// LoadRemotePackage pulls a CUE package from an OCI registry, resolves its
// transitive dependencies, and returns the built CUE value unified with the
// provided values map.
func (r *Renderer) LoadRemotePackage(ctx context.Context, modulePath, version, pkg string, values map[string]any) (cue.Value, error) {
	if err := r.initRegistry(); err != nil {
		return cue.Value{}, err
	}

	// Create a temp directory for the generated module file
	tmpDir, err := os.MkdirTemp("", "cue-render-*")
	if err != nil {
		return cue.Value{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate cue.mod/module.cue with resolved transitive deps
	if err := r.generateModuleFile(ctx, tmpDir, modulePath, version); err != nil {
		return cue.Value{}, err
	}

	// Load the remote package
	loadPath := modulePath + ":" + pkg
	instances := load.Instances([]string{loadPath}, &load.Config{
		Dir:      tmpDir,
		Registry: r.reg,
	})

	if len(instances) == 0 {
		return cue.Value{}, fmt.Errorf("no instances found for %s", loadPath)
	}

	inst := instances[0]
	if inst.Err != nil {
		return cue.Value{}, fmt.Errorf("failed to load package: %w", inst.Err)
	}

	value := r.ctx.BuildInstance(inst)
	if err := value.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to build instance: %w", err)
	}

	// Unify with values if provided
	if values != nil {
		valuesVal := r.ctx.Encode(values)
		if err := valuesVal.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("failed to encode values: %w", err)
		}

		value = value.Unify(valuesVal)
		if err := value.Err(); err != nil {
			return cue.Value{}, fmt.Errorf("failed to unify with values: %w", err)
		}
	}

	return value, nil
}

// generateModuleFile resolves transitive dependencies for a remote module
// and writes a cue.mod/module.cue file to dir.
func (r *Renderer) generateModuleFile(ctx context.Context, dir, modulePath, version string) error {
	mv, err := module.NewVersion(modulePath+"@v0", version)
	if err != nil {
		return fmt.Errorf("parsing module version: %w", err)
	}

	requirements, err := r.reg.Requirements(ctx, mv)
	if err != nil {
		return fmt.Errorf("fetching requirements: %w", err)
	}

	deps := map[string]*modfile.Dep{
		modulePath + "@v0": {
			Version: version,
			Default: true,
		},
	}
	for _, req := range requirements {
		deps[req.BasePath()] = &modfile.Dep{
			Version: req.Version(),
			Default: true,
		}
	}

	localModFile := &modfile.File{
		Module:   "conure.io/render@v0",
		Language: &modfile.Language{Version: "v0.15.1"},
		Deps:     deps,
	}

	data, err := modfile.Format(localModFile)
	if err != nil {
		return fmt.Errorf("formatting module.cue: %w", err)
	}

	modDir := filepath.Join(dir, "cue.mod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		return fmt.Errorf("creating cue.mod dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(modDir, "module.cue"), data, 0o644); err != nil {
		return fmt.Errorf("writing module.cue: %w", err)
	}

	return nil
}
