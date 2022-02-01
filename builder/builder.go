package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

type BuildConfig struct {
	OutputName string `hcl:"output_name,optional"`
	Source     string `hcl:"source,optional"`
}

type Builder struct {
	config BuildConfig
}

// Config - Implement Configurable
func (b *Builder) Config() (interface{}, error) {
	return &b.config, nil
}

// ConfigSet Implement ConfigurableNotify (we do config validation here)
func (b *Builder) ConfigSet(config interface{}) error {
	c, ok := config.(*BuildConfig)
	if !ok {
		return fmt.Errorf("expected *BuildConfig as parameter")
	}
	if _, err := os.Stat(c.Source); err != nil {
		return fmt.Errorf("source folder %v does not exist", c.Source)
	}
	return nil
}

// Implement Builder
func (b *Builder) BuildFunc() interface{} {
	// return a function which will be called by Waypoint
	return b.build
}

// A BuildFunc does not have a strict signature, you can define the parameters
// you need based on the Available parameters that the Waypoint SDK provides.
// Waypoint will automatically inject parameters as specified
// in the signature at run time.
//
// Available input parameters:
// - context.Context
// - *component.Source
// - *component.JobInfo
// - *component.DeploymentConfig
// - hclog.Logger
// - terminal.UI
// - *component.LabelSet
//
// The output parameters for BuildFunc must be a Struct which can
// be serialzied to Protocol Buffers binary format and an error.
// This Output Value will be made available for other functions
// as an input parameter.
// If an error is returned, Waypoint stops the execution flow and
// returns an error to the user.
func (b *Builder) build(ctx context.Context, ui terminal.UI) (*Binary, error) {
	u := ui.Status()
	defer u.Close()
	u.Update("Building application")

	if b.config.OutputName == "" {
		b.config.OutputName = "app"
	}
	if b.config.Source == "" {
		b.config.Source = "./"
	}
	c := exec.Command(
		"go",
		"build",
		"-o",
		b.config.OutputName,
		b.config.Source,
	)
	stdoutStderr, err := c.CombinedOutput()
	if err != nil {
		u.Step(terminal.StatusError, "Build failed no output")
	}

	u.Step(terminal.StatusOK, "Application built successfully " + string(stdoutStderr))
	return &Binary{
		Location: path.Join(b.config.Source, b.config.OutputName),
	}, nil
}
