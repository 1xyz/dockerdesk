project = "nginx-http-golang-dev"

# Labels can be specified for organizational purposes.
# labels = { "foo" = "bar" }


app "db" {
    config {
      env = {
        POSTGRES_USER = "postgres"
        POSTGRES_PASSWORD = "postgres"
        PG_DSN =  "host=db port=5432 user=postgres dbname=postgres sslmode=disable password=postgres"
      }
    }

    build {
      use "docker-pull" {
        image = "postgres"
        tag   = "13"
      }
    }

    deploy {
        use "dockerdesk" {
            published_ports = "5432:5432"
            use_app_as_container_name = true
        }
    }
}

app "backend" {
    path = "./backend"
    build {
        use "docker" {
        }
    }

    deploy {
        use "dockerdesk" {
            binds = ["${path.app}:/workspace:cached"]
            service_port = 80
            use_app_as_container_name = true
        }
    }
}

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