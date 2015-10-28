# terraform-provider-fleet

A plugin for Terraform enabling it to manipulate
[Fleet](https://github.com/coreos/fleet) (CoreOS) units.

[![Circle CI](https://circleci.com/gh/paperg/terraform-provider-fleet.svg?style=svg)](https://circleci.com/gh/paperg/terraform-provider-fleet)

## Installation

  1. Install [Terraform][1].
  2. `go get github.com/paperg/terraform-provider-fleet`

## Usage

This terraform plugin supports basic connections to the ETCD endpoint,
the HTTP API endpoint, and over SSH

There is minimal configuration currently supported so the ETCD and API clients
are attempting to connect directly without SSL

The configuration value 'driver' defaults to 'tunnel' but can be configured with:
 * etcd
 * api

EX:

```
provider "fleet" {
  driver = "etcd"
  endpoint = "http://192.168.0.1:4001"
}
```

```
provider "fleet" {
  driver = "api"
  endpoint = "http://192.168.0.1:4001"
}
```

There is only one resource: `fleet_unit`. Here is the first example from
[the Fleet introduction][3], transcribed to Terraform:

    provider "fleet" {
        endpoint = "IP_OR_HOSTNAME_OF_A_COREOS_HOST"
    }

    resource "fleet_unit" "myapp" {
        name = "myapp.service"
        desired_state = "launched" // "inactive", "loaded", or "launched"
        section {
            name = "Unit"

            option {
                name = "Description"
                value = "MyApp"
            }

            option {
                name = "After"
                value = "docker.service"
            }

            option {
                name = "Requires"
                value = "docker.service"
            }
        }

        section {
            name = "Service"

            option {
                name = "TimeoutStartSec"
                value = "0"
            }

            option {
                name = "ExecStartPre"
                value = "-/usr/bin/docker kill busybox2"
            }

            option {
                name = "ExecStartPre"
                value = "-/usr/bin/docker rm busybox2"
            }

            option {
                name = "ExecStartPre"
                value = "/usr/bin/docker pull busybox"
            }

            option {
                name = "ExecStart"
                value = "/usr/bin/docker run --name busybox2 busybox /bin/sh -c 'while true; do echo Hello World; sleep 1; done'"
            }

            option {
                name = "ExecStop"
                value = "/usr/bin/docker busybox2"
            }
        }
    }

## API stability

Both Terraform and Fleet are 0.x projects. Expect incompatible changes.


  [1]: https://terraform.io/
  [2]: https://terraform.io/docs/plugins/basics.html
  [3]: https://coreos.com/docs/launching-containers/launching/launching-containers-fleet/#run-a-container-in-the-cluster
