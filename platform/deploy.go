package platform

import (
	"context"
	"github.com/hashicorp/waypoint/builtin/docker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

type DeployConfig struct {
	Region string "hcl:directory,optional"
}

type Platform struct {
	config PlatformConfig
}

// Implement Configurable
func (p *Platform) Config() (interface{}, error) {
	return &p.config, nil
}

// Implement Builder
func (p *Platform) DeployFunc() interface{} {
	// return a function which will be called by Waypoint
	return p.Deploy
}

// StatusFunc implements component.Status
func (p *Platform) StatusFunc() interface{} {
	return p.Status
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

func (p *Platform) Deploy(
	ctx context.Context,
	log hclog.Logger,
	src *component.Source,
	job *component.JobInfo,
	img *docker.Image,
	deployConfig *component.DeploymentConfig,
	dcr *component.DeclaredResourcesResp,
	ui terminal.UI) (*docker.Deployment, error) {

	sg := ui.StepGroup()
	defer sg.Wait()

	if p.config.ServicePort == 0 {
		p.config.ServicePort = 3000
	}

	// Create our deployment and set an initial ID. This just creates
	// the initial structure this doesn't persist any state yet.
	var result docker.Deployment
	id, err := component.Id()
	if err != nil {
		return nil, err
	}
	result.Id = id
	result.Name = src.App

	// Create our resource manager and create
	rm := p.resourceManager(log, dcr)
	if err := rm.CreateAll(
		ctx, log, sg, ui,
		src, job, img, deployConfig, &result,
	); err != nil {
		return nil, err
	}

	// Store our resource state
	result.ResourceState = rm.State()

	// Get our container state
	crState := rm.Resource("container").State().(*docker.Resource_Container)
	if crState == nil {
		return nil, status.Errorf(codes.Internal,
			"container state is nil, this should never happen")
	}

	s := sg.Add("App deployed as container: " + crState.Name)
	s.Done()

	result.Container = crState.Id
	return &result, nil
}
