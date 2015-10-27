package main

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/ssh"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

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

// getAPI returns an API to Fleet.
func getAPI(driver string, driverEndpoint string, hostAddr string, maxRetries int) (client.API, error) {
	if hostAddr == "" {
		return nullAPI{}, nil
	}
	var endpoint *url.URL
	var dial func(network, addr string) (net.Conn, error)
	switch strings.ToLower(driver) {
	case "api":
		dial = net.Dial
	case "tunnel":
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
	return getAPI(driver, endpoint, addr, retries)
}
