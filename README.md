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

The configuration mimics the fleetctl tool.

Here are the configuration names, default values, and optional status from the
terraform provider schema:

```
schema.Schema{
    "driver": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "Adapter used to execute fleetctl commands. Options include api and etcd.",
        Default: "api",
    },
    "endpoint": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "Location of the fleet API if --driver=api. Alternatively, if --driver=etcd, location of the etcd API.",
        Default: "unix:///var/run/fleet.sock",
    },
    "etcd-key-prefix": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Default: registry.DefaultKeyPrefix,
        Description: "Keyspace for fleet data in etcd (development use only!)",
    },
    "key-file": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Default: "/var/run/fleet.sock",
        Description: "Location of TLS key file used to secure communication with the fleet API or etcd",
    },
    "cert-file": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "Location of TLS cert file used to secure communication with the fleet API or etcd",
    },
    "ca-file": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "Location of TLS CA file used to secure communication with the fleet API or etcd",
    },
    "tunnel": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "Establish an SSH tunnel through the provided address for communication with fleet and etcd.",
    },
    "known-hosts-file": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "File used to store remote machine fingerprints. Ignored if strict host key checking is disabled.",
        Default: ssh.DefaultKnownHostsFile,
    },
    "ssh-username": &schema.Schema{
        Type:     schema.TypeString,
        Optional: true,
        Description: "Username to use when connecting to CoreOS instance.",
        Default: "core",
    },

    "strict-host-key-checking": &schema.Schema{
        Type:     schema.TypeBool,
        Optional: true,
        Description: "Verify host keys presented by remote machines before initiating SSH connections.",
        Default: true,
    },

    "ssh-timeout": &schema.Schema{
        Type:     schema.TypeFloat,
        Optional: true,
        Description: "Amount of time in seconds to allow for SSH connection initialization before failing.",
        Default: 10.0,
    },
    "request-timeout": &schema.Schema{
        Type:     schema.TypeFloat,
        Optional: true,
        Description: "Amount of time in seconds to allow a single request before considering it failed.",
        Default: 3.0,
    },
}
```

EX:

```
provider "fleet" {
  driver = "etcd"
  endpoint = "http://192.168.0.1:4001"
  // etcd_key_prefix can be ommited to use the default value
  etcd_key_prefix = "/_some/_weird/etcd/prefix"
  // connection_retries defaults to 12
  connection_retries = 9000
}
```

```
provider "fleet" {
  driver = "api"
  endpoint = "http://192.168.0.1:8080"
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
