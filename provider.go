package main

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/registry"
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
func getAPI(hostAddr string, maxRetries int) (client.API, error) {
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

	dial := func(network, addr string) (net.Conn, error) {
		tcpaddr, err := net.ResolveTCPAddr(network, addr)
		if err != nil {
			return nil, err
		}
		return sshClient.DialTCP(network, nil, tcpaddr)
	}

	trans := &http.Transport{
		Dial: dial,
	}

	etcdClient, err := etcd.NewClient(
		[]string{"http://127.0.0.1:4001"},
		trans,
		time.Minute,
	)
	if err != nil {
		return nil, err
	}

	reg := registry.NewEtcdRegistry(etcdClient, registry.DefaultKeyPrefix)

	return &client.RegistryClient{Registry: reg}, nil
}

// Provider returns the ResourceProvider implemented by this package. Serve
// this with the Terraform plugin helper and you are golden.
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"tunnel_address": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
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
	return getAPI(addr, retries)
}
