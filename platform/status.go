package platform

import (
	"context"
	"github.com/hashicorp/go-hclog"
	sdk "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"github.com/hashicorp/waypoint/builtin/docker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
)

var (
	mixedHealthWarn = strings.TrimSpace(`
Waypoint detected that the current deployment is not ready, however your application
might be available or still starting up.
`)
)

func (p *Platform) Status(
	ctx context.Context,
	log hclog.Logger,
	deployment *docker.Deployment,
	ui terminal.UI,
) (*sdk.StatusReport, error) {
	cli, err := p.getDockerClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "unable to create Docker client: %s", err)
	}
	cli.NegotiateAPIVersion(ctx)

	sg := ui.StepGroup()
	defer sg.Wait()

	s := sg.Add("Gathering health report for Docker platform...")
	defer s.Abort()

	rm := p.resourceManager(log, nil)

	// If we don't have resource state, this state is from an older version
	// and we need to manually recreate it.
	if deployment.ResourceState == nil {
		if err := rm.Resource("container").SetState(&docker.Resource_Container{
			Id: deployment.Container,
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to manually set container resource state while restoring from an old deployment: %s", err)
		}
		if err := rm.Resource("network").SetState(&docker.Resource_Network{
			Name: "waypoint",
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to manually set network resource state while restoring from an old deployment: %s", err)
		}
	} else {
		// Load our set state
		if err := rm.LoadState(deployment.ResourceState); err != nil {
			return nil, status.Errorf(codes.Internal, "resource manager failed to load state: %s", err)
		}
	}

	result, err := rm.StatusReport(ctx, log, sg, cli, ui)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resource manager failed to generate resource statuses: %s", err)
	}

	log.Debug("status report complete")

	// update output based on main health state
	s.Update("Finished building report for Docker platform")
	s.Done()

	// NOTE(briancain): Replace ui.Status with StepGroups once this bug
	// has been fixed: https://github.com/hashicorp/waypoint/issues/1536
	st := ui.Status()
	defer st.Close()

	st.Update("Determining overall container health...")
	if result.Health == sdk.StatusReport_READY {
		st.Step(terminal.StatusOK, result.HealthMessage)
	} else {
		if result.Health == sdk.StatusReport_PARTIAL {
			st.Step(terminal.StatusWarn, result.HealthMessage)
		} else {
			st.Step(terminal.StatusError, result.HealthMessage)
		}

		// Extra advisory wording to let user know that the deployment could be still starting up
		// if the report was generated immediately after it was deployed or released.
		st.Step(terminal.StatusWarn, mixedHealthWarn)
	}

	return result, nil
}
