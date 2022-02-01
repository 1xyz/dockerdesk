PLUGIN_NAME=dockerdesk

.PHONY: all

all: build

# Builds the plugin on your local machine
build:
	@echo ""
	@echo "Compile Plugin"

	# Clear the output
	rm -rf ./bin

	go mod tidy
	go build -o ./bin/waypoint-plugin-${PLUGIN_NAME} ./main.go
	GOOS=linux GOARCH=amd64 go build -o ./bin/linux_amd64/waypoint-plugin-${PLUGIN_NAME} ./main.go
	GOOS=darwin GOARCH=amd64 go build -o ./bin/darwin_amd64/waypoint-plugin-${PLUGIN_NAME} ./main.go
	GOOS=windows GOARCH=amd64 go build -o ./bin/windows_amd64/waypoint-plugin-${PLUGIN_NAME}.exe ./main.go

# Install the plugin locally
install:
	@echo ""
	@echo "Installing Plugin"

	cp ./bin/waypoint-plugin-${PLUGIN_NAME} ${HOME}/.config/waypoint/plugins/
	cp ./bin/linux_amd64/waypoint-plugin-${PLUGIN_NAME}* ${HOME}/.config/waypoint/plugins/
	cp ./bin/darwin_amd64/waypoint-plugin-${PLUGIN_NAME}* ${HOME}/.config/waypoint/plugins/
	cp ./bin/windows_amd64/waypoint-plugin-${PLUGIN_NAME}* ${HOME}/.config/waypoint/plugins/

# Zip the built plugin binaries
zip:
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_linux_amd64.zip ./bin/linux_amd64/waypoint-plugin-${PLUGIN_NAME}
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_darwin_amd64.zip ./bin/darwin_amd64/waypoint-plugin-${PLUGIN_NAME}
	zip -j ./bin/waypoint-plugin-${PLUGIN_NAME}_windows_amd64.zip ./bin/windows_amd64/waypoint-plugin-${PLUGIN_NAME}.exe

# Build the plugin using a Docker container
build-docker:
	rm -rf ./releases
	DOCKER_BUILDKIT=1 docker build --output releases --progress=plain .