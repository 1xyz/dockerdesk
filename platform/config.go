package platform


// PlatformConfig is the configuration structure for the Platform.
type PlatformConfig struct {
	// A list of folders to mount to the container.
	Binds []string `hcl:"binds,optional"`

	// ClientConfig allow the user to specify the connection to the Docker
	// engine. By default we try to load this from env vars:
	// DOCKER_HOST to set the url to the docker server.
	// DOCKER_API_VERSION to set the version of the API to reach, leave empty for latest.
	// DOCKER_CERT_PATH to load the TLS certificates from.
	// DOCKER_TLS_VERIFY to enable or disable TLS verification, off by default.
	ClientConfig *ClientConfig `hcl:"client_config,block"`

	// The command to run in the container. This is an array of arguments
	// that are executed directly. These are not executed in the context of
	// a shell. If you want to use a shell, add that to this command manually.
	Command []string `hcl:"command,optional"`

	// Force pull the image from the remote repository
	ForcePull bool `hcl:"force_pull,optional"`

	// A map of key/value pairs, stored in docker as a string. Each key/value pair must
	// be unique. Validiation occurs at the docker layer, not in Waypoint. Label
	// keys are alphanumeric strings which may contain periods (.) and hyphens (-).
	// See the docker docs for more info: https://docs.docker.com/config/labels-custom-metadata/
	Labels map[string]string `hcl:"labels,optional"`

	// An array of strings with network names to connect the container to
	Networks []string `hcl:"networks,optional"`

	// A map of resources to configure the container with such as memory and cpu
	// limits.
	Resources map[string]string `hcl:"resources,optional"`

	// A path to a directory that will be created for the service to store
	// temporary data.
	ScratchSpace string `hcl:"scratch_path,optional"`

	// Environment variables that are meant to configure the application in a static
	// way. This might be control an image that has mulitple modes of operation,
	// selected via environment variable. Most configuration should use the waypoint
	// config commands.
	StaticEnvVars map[string]string `hcl:"static_environment,optional"`

	// Additional ports the application is listening on to expose on the container
	ExtraPorts []uint `hcl:"extra_ports,optional"`

	// Port that your service is running on within the actual container.
	// Defaults to port 3000.
	// TODO Evaluate if this should remain as a default 3000, should be a required field,
	// or default to another port.
	ServicePort uint `hcl:"service_port,optional"`

	// PublishedPorts is a CSV of docker ports published
	// - Each entry is of the form <container-port>:<host=port>/<proto>
	//   where host-port and proto are optional
	// - Example: 3000:3001/tcp, 8080:80
	// See: https://docs.docker.com/config/containers/container-networking/#published-ports
	PublishedPorts string `hcl:"published_ports,optional"`
}

type ClientConfig struct {
	// Host to use when connecting to Docker
	// This can be used to connect to remote Docker instances
	Host string `hcl:"host,optional"`

	// Path to load the certificates for the Docker Engine
	CertPath string `hcl:"cert_path,optional"`

	// Docker API version to use for connection
	APIVersion string `hcl:"api_version,optional"`
}

