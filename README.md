# dockerdesk 

Experimental Docker desktop waypoint plugin (platform/deploy plugin)

## Notes

- The docker desktop plugin is a fork from waypoint's [builtin docker plugin](https://pkg.go.dev/github.com/hashicorp/waypoint/builtin/docker).
- Ability to pin ports and container names, much like docker-compose.
- Why? The plugin gives the flexibility to expose the services I am building right from the desktop within my team.

## Build
```shell
make
```

## Install
```shell
$ export XDG_CONFIG_HOME=$HOME/.config/waypoint
$ make install
```

## Example use within a project

In your project, ensure

* you have the plugin binary placed in `$HOME/.config/waypoint/plugins`. atleast on osx and linux
* `export XDG_CONFIG_HOME=$HOME/.config/waypoint`
* Here is an example `waypoint.hcl` where the `dockerdesk` plugin is used

```
app "frontend" {
    path = "./frontend"
    build {
        use "docker" {
        }
    }

    deploy {
        use "dockerdesk" {
            published_ports = "80:80" 
            service_port = 80
            use_app_as_container_name = true
        }
    }
}
```

## Resources
* How to a write plugin [here](https://www.waypointproject.io/docs/extending-waypoint/creating-plugins)
* For a working example see [here](./_examples/example_waypoint.hcl)