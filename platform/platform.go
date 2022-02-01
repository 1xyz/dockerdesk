package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/go-connections/nat"
	goUnits "github.com/docker/go-units"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/framework/resource"
	sdk "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"github.com/hashicorp/waypoint/builtin/docker"
	wpdockerclient "github.com/hashicorp/waypoint/builtin/docker/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	platformName = "dockerdev"
	labelId      = "waypoint.hashicorp.com/id"
	labelNonce   = "waypoint.hashicorp.com/nonce"
)

func (p *Platform) resourceManager(log hclog.Logger, dcr *component.DeclaredResourcesResp) *resource.Manager {
	return resource.NewManager(
		resource.WithLogger(log.Named("resource_manager")),
		resource.WithValueProvider(p.getDockerClient),
		resource.WithDeclaredResourcesResp(dcr),
		resource.WithResource(resource.NewResource(
			resource.WithName("network"),
			resource.WithState(&docker.Resource_Network{}),
			resource.WithCreate(p.resourceNetworkCreate),
			// networks have no destroy logic, we leave the network
			// lingering around for now. This was the logic prior to
			// refactoring into the resource manager so we kept it.

			resource.WithStatus(p.resourceNetworkStatus),
			resource.WithPlatform(platformName),
			resource.WithCategoryDisplayHint(sdk.ResourceCategoryDisplayHint_ROUTER), // Not a perfect fit but good enough.
		)),

		resource.WithResource(resource.NewResource(
			resource.WithName("container"),
			resource.WithState(&docker.Resource_Container{}),
			resource.WithCreate(p.resourceContainerCreate),
			resource.WithDestroy(p.resourceContainerDestroy),
			resource.WithStatus(p.resourceContainerStatus),
			resource.WithPlatform(platformName),
			resource.WithCategoryDisplayHint(sdk.ResourceCategoryDisplayHint_INSTANCE),
		)),
	)
}

func (p *Platform) getDockerClient(ctx context.Context) (*client.Client, error) {
	if p.config.ClientConfig == nil {
		return wpdockerclient.NewClientWithOpts(client.FromEnv)
	}
	opts := []client.Opt{}
	if host := p.config.ClientConfig.Host; host != "" {
		opts = append(opts, client.WithHost(host))
	}

	if path := p.config.ClientConfig.CertPath; path != "" {
		opts = append(opts, client.WithTLSClientConfig(
			filepath.Join(path, "ca.pem"),
			filepath.Join(path, "cert.pem"),
			filepath.Join(path, "key.pem"),
		))
	}

	if version := p.config.ClientConfig.APIVersion; version != "" {
		opts = append(opts, client.WithVersion(version))
	}

	cli, err := wpdockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	cli.NegotiateAPIVersion(ctx)
	return cli, nil
}

func (p *Platform) resourceNetworkCreate(
	ctx context.Context,
	cli *client.Client,
	sg terminal.StepGroup,
	state *docker.Resource_Network,
) error {
	s := sg.Add("Setting up network...")
	defer func() { s.Abort() }()

	nets, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "use=waypoint")),
	})
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "unable to list Docker networks: %s", err)
	}

	// If we have a network already we're done. If we don't have a net, create it.
	if len(nets) == 0 {
		_, err = cli.NetworkCreate(ctx, "waypoint", types.NetworkCreate{
			Driver:         "bridge",
			CheckDuplicate: true,
			Internal:       false,
			Attachable:     true,
			Labels: map[string]string{
				"use": "waypoint",
			},
		})
		if err != nil {
			return status.Errorf(codes.FailedPrecondition, "unable to create Docker network: %s", err)
		}
	}
	s.Done()

	// Set our state
	state.Name = "waypoint"

	return nil
}

func (p *Platform) resourceNetworkStatus(
	ctx context.Context,
	log hclog.Logger,
	sg terminal.StepGroup,
	cli *client.Client,
	network *docker.Resource_Network,
	sr *resource.StatusResponse,
) error {
	s := sg.Add("Checking status of the Docker network resource...")
	defer s.Abort()

	log.Debug("querying docker for network status")

	nets, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("label", fmt.Sprintf("use=%s", network.Name))),
	})
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "unable to list Docker networks: %s", err)
	}
	if len(nets) == 0 {
		sr.Resources = append(sr.Resources, &sdk.StatusReport_Resource{
			Name:                network.Name,
			CategoryDisplayHint: sdk.ResourceCategoryDisplayHint_ROUTER,
			Health:              sdk.StatusReport_MISSING,
		})
	} else {
		// There shouldn't be multiple networks, but if there are somehow we should show them
		for _, net := range nets {
			netJson, err := json.Marshal(map[string]interface{}{
				"dockerNetwork": net,
			})
			if err != nil {
				return status.Errorf(codes.FailedPrecondition, "failed to marshal docker network status for network with id %q: %s", net.ID, err)
			}
			sr.Resources = append(sr.Resources, &sdk.StatusReport_Resource{
				Name:                net.Name,
				Id:                  net.ID,
				CategoryDisplayHint: sdk.ResourceCategoryDisplayHint_ROUTER,
				Health:              sdk.StatusReport_READY, // Not that many states a network can be in, if it exists.
				HealthMessage:       "exists",
				StateJson:           string(netJson),
				CreatedTime:         timestamppb.New(net.Created),
			})
		}
	}

	s.Update("Finished building report for Docker network resource")
	s.Done()
	return nil
}

func (p *Platform) resourceContainerCreate(
	ctx context.Context,
	log hclog.Logger,
	cli *client.Client,
	src *component.Source,
	img *docker.Image,
	job *component.JobInfo,
	deployConfig *component.DeploymentConfig,
	result *docker.Deployment,
	sg terminal.StepGroup,
	ui terminal.UI,
	state *docker.Resource_Container,
	netState *docker.Resource_Network,
) error {
	// Pull the image
	err := p.pullImage(cli, log, ui, img, p.config.ForcePull)
	if err != nil {
		return status.Errorf(codes.FailedPrecondition,
			"unable to pull image from Docker registry: %s", err)
	}

	s := sg.Add("Creating new container...")
	defer func() { s.Abort() }()

	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	pfs, err := parsePublishPorts(p.config.PublishedPorts)
	if err != nil {
		return fmt.Errorf("parsePublishedPort %v %w", p.config.PublishedPorts, err)
	}
	for i := range pfs {
		pf := pfs[i]
		np, err := nat.NewPort(pf.Proto, pf.ContainerPort)
		if err != nil {
			return fmt.Errorf("createPort from %v err = %w", pf, err)
		}
		exposedPorts[np] = struct{}{}
		portBindings[np] = []nat.PortBinding{
			{
				HostPort: pf.HostPort,
			},
		}
	}

	for _, port := range append(p.config.ExtraPorts, p.config.ServicePort) {
		np, err := nat.NewPort("tcp", fmt.Sprint(port))
		if err != nil {
			return err
		}

		exposedPorts[np] = struct{}{}
		portBindings[np] = []nat.PortBinding{
			{
				HostPort: "", // this is intentionally left empty for a random host port assignment
			},
		}
	}

	cfg := container.Config{
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		OpenStdin:    true,
		StdinOnce:    true,
		Image:        img.Image + ":" + img.Tag,
		ExposedPorts: exposedPorts,
		Env:          []string{"PORT=" + fmt.Sprint(p.config.ServicePort)},
	}
	if c := p.config.Command; len(c) > 0 {
		cfg.Cmd = c
	}

	// default container binds
	containerBinds := []string{src.App + "-scratch" + ":/input"}
	if p.config.Binds != nil {
		containerBinds = append(containerBinds, p.config.Binds...)
	}

	// Setup the resource requirements for the container if given
	var resources container.Resources
	if p.config.Resources != nil {
		memory, err := goUnits.FromHumanSize(p.config.Resources["memory"])
		if err != nil {
			return err
		}
		resources.Memory = memory

		cpu, err := strconv.ParseInt(p.config.Resources["cpu"], 10, 64)
		if err != nil {
			return err
		}
		resources.CPUShares = cpu
	}

	// Build our host configuration from the bindings, ports, and resources.
	hostconfig := container.HostConfig{
		Binds:        containerBinds,
		PortBindings: portBindings,
		Resources:    resources,
	}

	// Containers can only be connected to 1 network at creation time
	// Additional user defined networks will be connected after container is
	// created.
	netconfig := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			netState.Name: {},
		},
	}

	for k, v := range p.config.StaticEnvVars {
		cfg.Env = append(cfg.Env, k+"="+v)
	}
	for k, v := range deployConfig.Env() {
		cfg.Env = append(cfg.Env, k+"="+v)
	}

	// Setup the labels. We setup a set of defaults and then override them
	// with any user configured labels.
	defaultLabels := map[string]string{
		labelId:     result.Id,
		"app":       src.App,
		"workspace": job.Workspace,
	}
	if p.config.Labels != nil {
		for k, v := range defaultLabels {
			p.config.Labels[k] = v
		}
	} else {
		p.config.Labels = defaultLabels
	}
	cfg.Labels = p.config.Labels

	// Create the container
	name := src.App + "-" + result.Id
	if p.config.UseAppAsContainerName {
		name = src.App
	}
	cr, err := cli.ContainerCreate(ctx, &cfg, &hostconfig, &netconfig, nil, name)
	if err != nil {
		return status.Errorf(codes.Internal, "unable to create Docker container: %s", err)
	}

	// Store our state so we can destroy it properly
	state.Id = cr.ID
	state.Name = name

	// Additional networks must be connected after container is created
	if p.config.Networks != nil {
		s.Update("Connecting additional networks to container...")
		for _, net := range p.config.Networks {
			err = cli.NetworkConnect(ctx, net, cr.ID, &network.EndpointSettings{})
			if err != nil {
				s.Update("Failed to connect additional network")
				s.Status(terminal.StatusError)
				s.Done()
				return status.Errorf(
					codes.Internal,
					"unable to connect container to additional networks: %s",
					err)
			}
		}
	}

	s.Update("Starting container")
	err = cli.ContainerStart(ctx, cr.ID, types.ContainerStartOptions{})
	if err != nil {
		return status.Errorf(codes.Internal, "unable to start Docker container: %s", err)
	}
	s.Done()

	return nil
}

func (p *Platform) pullImage(cli *client.Client, log hclog.Logger, ui terminal.UI, img *docker.Image, force bool) error {
	in := fmt.Sprintf("%s:%s", img.Image, img.Tag)
	args := filters.NewArgs()
	args.Add("reference", in)

	sg := ui.StepGroup()
	s := sg.Add("")
	defer func() { s.Abort() }()

	// only pull if image is not in current registry so check to see if the image is present
	// if force then skip this check
	if force == false {
		s.Update("Checking Docker image cache for Image " + in)

		sum, err := cli.ImageList(context.Background(), types.ImageListOptions{Filters: args})
		if err != nil {
			return fmt.Errorf("unable to list images in local Docker cache: %w", err)
		}

		// if we have images do not pull
		if len(sum) > 0 {
			s.Update("Docker image %q up to date!", in)
			s.Done()
			return nil
		}
	}

	s.Update("Pulling Docker Image " + in)

	ipo := types.ImagePullOptions{}

	// if the username and password is not null make an authenticated
	// image pull
	/*
		if image.Username != "" && image.Password != "" {
			ipo.RegistryAuth = createRegistryAuth(image.Username, image.Password)
		}
	*/

	named, err := reference.ParseNormalizedNamed(in)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "unable to parse image name: %s", in)
	}

	in = named.Name()
	log.Debug("pulling image", "image", in)

	out, err := cli.ImagePull(context.Background(), in, ipo)
	if err != nil {
		return fmt.Errorf("unable to pull image: %w", err)
	}

	stdout, _, err := ui.OutputWriters()
	if err != nil {
		return fmt.Errorf("unable to get output writers: %s", err)
	}

	var termFd uintptr
	if f, ok := stdout.(*os.File); ok {
		termFd = f.Fd()
	}

	err = jsonmessage.DisplayJSONMessagesStream(out, s.TermOutput(), termFd, true, nil)
	if err != nil {
		return status.Errorf(codes.Internal, "unable to stream build logs to the terminal: %s", err)
	}

	s.Done()

	return nil
}

func (p *Platform) resourceContainerDestroy(
	ctx context.Context,
	cli *client.Client,
	state *docker.Resource_Container,
	sg terminal.StepGroup,
) error {
	// Check if the container exists
	_, err := cli.ContainerInspect(ctx, state.Id)
	if client.IsErrNotFound(err) {
		return nil
	}

	s := sg.Add("Deleting container: %s", state.Id)
	defer func() { s.Abort() }()

	// Remove it
	err = cli.ContainerRemove(ctx, state.Id, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return err
	}

	s.Done()
	return nil
}

func (p *Platform) resourceContainerStatus(
	ctx context.Context,
	log hclog.Logger,
	sg terminal.StepGroup,
	ui terminal.UI,
	cli *client.Client,
	container *docker.Resource_Container,
	sr *resource.StatusResponse,
) error {
	s := sg.Add("Checking status of the Docker container resource...")
	defer s.Abort()

	log.Debug("querying docker for container health")

	// Creating our baseline container resource
	containerResource := &sdk.StatusReport_Resource{
		CategoryDisplayHint: sdk.ResourceCategoryDisplayHint_INSTANCE,
	}

	// Add the container resource to the the status response. After this function finishes,
	// the resource manager framework will read the return value out of here.
	sr.Resources = append(sr.Resources, containerResource)

	// NOTE(briancain): The docker platform currently only deploys a single
	// container, so for now the status report makes the same assumption.
	containerInfo, err := cli.ContainerInspect(ctx, container.Id)
	if err != nil {
		if client.IsErrNotFound(err) {
			// We expected this container to be present, but it's not.
			// It has likely been killed and removed from the platform since it was deployed.
			containerResource.Name = container.Name
			containerResource.Id = container.Id
			containerResource.Health = sdk.StatusReport_MISSING
		} else {
			return status.Errorf(codes.FailedPrecondition, "error quering docker for container status: %s", err)
		}
	} else {
		// Add everything that docker knows about the running container to the container resource.
		log.Debug("Found docker container", "id", container.Id)

		containerResource.Id = containerInfo.ID

		// NOTE(izaak): Docker container names officially begin with "/" when running on the local daemon, but the
		// docker CLI strips the leading forward slash, so we'll do that too.
		containerResource.Name = strings.TrimPrefix(containerInfo.Name, "/")

		containerCreatedTime, err := time.Parse(time.RFC3339, containerInfo.Created)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to parse docker timestamp %q: %s", containerInfo.Created, err)
		}
		containerResource.CreatedTime = timestamppb.New(containerCreatedTime)

		// Figure out the resource state based on container state or health
		if containerInfo.State.Health != nil {
			// Built-in Docker health reporting
			// NOTE: this only works if the container has configured health checks

			switch containerInfo.State.Health.Status {
			case "Healthy":
				containerResource.Health = sdk.StatusReport_READY
				containerResource.HealthMessage = "container is running"
			case "Unhealthy":
				containerResource.Health = sdk.StatusReport_DOWN
				containerResource.HealthMessage = "container is down"
			case "Starting":
				containerResource.Health = sdk.StatusReport_ALIVE
				containerResource.HealthMessage = "container is starting"
			default:
				containerResource.Health = sdk.StatusReport_UNKNOWN
				containerResource.HealthMessage = "unknown status reported by docker for container"
			}
		} else {
			// Waypoint container inspection
			if containerInfo.State.Running && containerInfo.State.ExitCode == 0 {
				containerResource.Health = sdk.StatusReport_READY
				containerResource.HealthMessage = "container is running"
			} else if containerInfo.State.Restarting || containerInfo.State.Status == "created" {
				containerResource.Health = sdk.StatusReport_ALIVE
				containerResource.HealthMessage = "container is still starting"
			} else if containerInfo.State.Dead || containerInfo.State.OOMKilled || containerInfo.State.ExitCode != 0 {
				containerResource.Health = sdk.StatusReport_DOWN
				containerResource.HealthMessage = "container is down"
			} else {
				containerResource.Health = sdk.StatusReport_UNKNOWN
				containerResource.HealthMessage = "unknown status for container"
			}
		}

		// Redact container env vars, which can contain secrets
		containerInfo.Config.Env = []string{}

		containerState := map[string]interface{}{
			"dockerContainerInfo": containerInfo,
		}

		// Pull out some useful common fields if we can
		if containerInfo.NetworkSettings != nil {
			if len(containerInfo.NetworkSettings.Networks) == 1 { // we should only have one network (called "waypoint")
				for _, dockerNetwork := range containerInfo.NetworkSettings.Networks {
					containerState["ipAddress"] = dockerNetwork.IPAddress
				}
			}
		}

		stateJson, err := json.Marshal(containerState)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to marshal container info to json: %s", err)
		}
		containerResource.StateJson = string(stateJson)
	}

	s.Update("Finished building report for Docker container resource")
	s.Done()
	return nil
}

type portField struct {
	ContainerPort string
	HostPort      string
	Proto         string
}

func (pf *portField) String() string {
	return fmt.Sprintf("ContainerPort=%s HostPort=%s Proto=%s",
		pf.ContainerPort, pf.HostPort, pf.Proto)
}

func parsePublishPorts(portValueCSV string) ([]*portField, error) {
	pfs := make([]*portField, 0)
	if len(portValueCSV) == 0 {
		return pfs, nil
	}

	tok := strings.Split(portValueCSV, ",")
	for i := range tok {
		pf, err := parsePublishPortField(tok[i])
		if err != nil {
			return nil, err
		}
		pfs = append(pfs, pf)
	}
	return pfs, nil
}

func parsePublishPortField(portValue string) (*portField, error) {
	tok := strings.Split(portValue, ":")
	if len(tok) == 1 {
		containerPort, proto, err := parseProtoField(tok[0])
		if err != nil {
			return nil, err
		}
		return &portField{containerPort, "", proto}, nil
	}

	if len(tok) == 2 {
		containerPort := tok[0]
		hostPort, proto, err := parseProtoField(tok[1])
		if err != nil {
			return nil, err
		}
		return &portField{containerPort, hostPort, proto}, nil
	}

	return nil, fmt.Errorf("invalid port field %v", portValue)
}

func parseProtoField(v string) (port string, proto string, err error) {
	port = ""
	proto = "tcp"
	err = nil

	tok := strings.Split(v, "/")
	if len(tok) == 1 {
		port = tok[0]
		return
	}

	if len(tok) == 2 {
		port = tok[0]
		proto = tok[1]
		return
	}

	err = fmt.Errorf("invalid format. %v", v)
	return
}
