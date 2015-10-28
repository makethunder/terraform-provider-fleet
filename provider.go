package main

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"fmt"
	"log"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/ssh"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"

	"github.com/coreos/fleet/version"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/etcd"
)

const oldVersionWarning = `####################################################################
WARNING: fleetctl (%s) is older than the latest registered
version of fleet found in the cluster (%s). You are strongly
recommended to upgrade fleetctl to prevent incompatibility issues.
####################################################################
`

// retry wraps a function with retry logic. Only errors containing "timed out"
// will be retried. (authentication errors and other stuff should fail
// immediately)
func retry(f func() (interface{}, error), maxRetries int) (interface{}, error) {
	var result interface{}
	var err error

	for retries := 0; retries < maxRetries; retries++ {
		result, err = f()
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			break
		}
	}
	return result, err
}

func checkVersion(cReg registry.ClusterRegistry) (string, bool) {
	fv := version.SemVersion
	lv, err := cReg.LatestDaemonVersion()
	if err != nil {
		log.Fatal("error attempting to check latest fleet version in Registry: %v", err)
	} else if lv != nil && fv.LessThan(*lv) {
		return fmt.Sprintf(oldVersionWarning, fv.String(), lv.String()), false
	}
	return "", true
}

// getAPI returns an API to Fleet.
func getAPI(driver string, driverEndpoint string, hostAddr string, maxRetries int, etcdKeyPrefix string) (client.API, error) {

	var endpoint *url.URL
	var dial func(network, addr string) (net.Conn, error)
	switch strings.ToLower(driver) {
	case "api":
		log.Printf("Using API connection for requests")
		dial = net.Dial
	case "etcd":
		timeout := 20000 * time.Millisecond
		trans := &http.Transport{}
		machines := strings.Split(driverEndpoint, ",")
		eClient, err := etcd.NewClient(machines, trans, timeout)
		if err != nil {
			return nil, err
		}

		reg := registry.NewEtcdRegistry(eClient, etcdKeyPrefix)

		if msg, ok := checkVersion(reg); !ok {
			log.Fatal(msg)
		}

		return &client.RegistryClient{Registry: reg}, nil
	case "tunnel":
		if hostAddr == "" {
			return nullAPI{}, nil
		}

		getSSHClient := func() (interface{}, error) {
			return ssh.NewSSHClient("core", hostAddr, nil, false, time.Second*10)
		}

		result, err := retry(getSSHClient, maxRetries)
		if err != nil {
			return nil, err
		}
		sshClient := result.(*ssh.SSHForwardingClient)

		dial = func(string, string) (net.Conn, error) {
			cmd := "fleetctl fd-forward /var/run/fleet.sock"
			return ssh.DialCommand(sshClient, cmd)
		}
	default:
		return nullAPI{}, nil
	}

	endpoint, err := url.Parse(driverEndpoint)

	if err != nil {
		return nil, err
	}

	trans := pkg.LoggingHTTPTransport{
		Transport: http.Transport{
			Dial: dial,
		},
	}

	httpClient := http.Client{
		Transport: &trans,
	}

	return client.NewHTTPClient(&httpClient, *endpoint)
}

// Provider returns the ResourceProvider implemented by this package. Serve
// this with the Terraform plugin helper and you are golden.
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"driver": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default: "tunnel",
				Description: "Driver to use to connect to Fleet. Can be tunnel or api.",
			},
			"tunnel_address": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"endpoint": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default: "http://domain-sock",
				Description: "Endpoint for Fleet. You do not need to set this if you are using SSH.",
			},
			"connection_retries": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  12,
			},
			"etc_key_prefix": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default: "/_coreos.com/fleet/",
				Description: "EtcdKeyPrefix to use for fleet",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"fleet_unit": resourceUnit(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	addr := d.Get("tunnel_address").(string)
	retries := d.Get("connection_retries").(int)
	driver := d.Get("driver").(string)
	endpoint := d.Get("endpoint").(string)
	etcKeyPrefix := d.Get("etc_key_prefix").(string)
	return getAPI(driver, endpoint, addr, retries, etcKeyPrefix)
}
