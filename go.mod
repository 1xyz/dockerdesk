module github.com/aardlabs/dockerdesk

go 1.16

require (
	github.com/hashicorp/waypoint-plugin-sdk v0.0.0-20211012192505-5c78341a47e4
	google.golang.org/protobuf v1.26.0
)

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0
	github.com/hashicorp/go-hclog v0.16.1
	github.com/hashicorp/waypoint v0.7.1
	google.golang.org/grpc v1.39.1
)

// replace github.com/hashicorp/waypoint-plugin-sdk => ../../waypoint-plugin-sdk
