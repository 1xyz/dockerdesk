# dockerdesk 

Docker desktop waypoint plugin (platform/deploy)

## Build
```shell
make && make install
```
## Example use within a project

In your project, ensure

* you have the plugin binary placed in `$HOME/.config/waypoint/plugins`. atleast on osx and linux
* `export XDG_CONFIG_HOME=$HOME/.config/waypoint`
* Here is an example `waypoint.hcl` where the `dockerdesk` plugin is used

```json
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

For a working example see [here](https://github.com/aardlabs/nginx-gohttp-dev/blob/main/waypoint.hcl)