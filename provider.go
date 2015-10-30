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

	etcd "github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/etcd/client"
)

const oldVersionWarning = `####################################################################
WARNING: The linked against fleet go lib (%s) is older than the latest
registered version of fleet found in the cluster (%s). You are strongly
recommended to upgrade the linked fleet go lib and rebuild to prevent
incompatibility issues.
####################################################################
`

const defaultTimeout = time.Second*10

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

func getHTTPClient(driverEndpoint string) (client.API, error) {
	log.Printf("Using API connection for requests")

	endpoint, err := url.Parse(driverEndpoint)

	if err != nil {
		return nil, err
	}

	trans := pkg.LoggingHTTPTransport{
		Transport: http.Transport{},
	}

	httpClient := http.Client{
		Transport: &trans,
	}

	return client.NewHTTPClient(&httpClient, *endpoint)
}

func getETCDClient(driverEndpoint string, etcdKeyPrefix string) (client.API, error) {
	log.Printf("Using ETCD connection for requests")

	trans := &http.Transport{}

	eCfg := etcd.Config{
		Endpoints: strings.Split(driverEndpoint, ","),
		Transport: trans,
	}

	eClient, err := etcd.New(eCfg)
	if err != nil {
		return nil, err
	}

	kAPI := etcd.NewKeysAPI(eClient)
	reg := registry.NewEtcdRegistry(kAPI, etcdKeyPrefix, defaultTimeout)

	if msg, ok := checkVersion(reg); !ok {
		log.Printf(msg)
	}

	return &client.RegistryClient{Registry: reg}, nil
}

func getTunnelClient(driverEndpoint string, maxRetries int) (client.API, error) {
	log.Printf("Using Fleet Tunnel connection for requests")

	getSSHClient := func() (interface{}, error) {
		return ssh.NewSSHClient("core", driverEndpoint, nil, false, defaultTimeout)
	}

	result, err := retry(getSSHClient, maxRetries)
	if err != nil {
		return nil, err
	}
	sshClient := result.(*ssh.SSHForwardingClient)

	dial := func(string, string) (net.Conn, error) {
		cmd := "fleetctl fd-forward /var/run/fleet.sock"
		return ssh.DialCommand(sshClient, cmd)
	}

	// This is needed to fake out the client - it isn't used
	// since we're overloading the dial method on the transport
	// but the client complains if it isn't set
	fakeHttpEndpoint, err := url.Parse("http://domain-sock")

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

	return client.NewHTTPClient(&httpClient, *fakeHttpEndpoint)
}


// getAPI returns an API to Fleet.
func getAPI(driver string, driverEndpoint string, maxRetries int, etcdKeyPrefix string) (client.API, error) {

	switch strings.ToLower(driver) {
	case "api":
		return getHTTPClient(driverEndpoint)
	case "etcd":
		return getETCDClient(driverEndpoint, etcdKeyPrefix)
	case "tunnel":
		if len(driverEndpoint) > 0 {
			return getTunnelClient(driverEndpoint, maxRetries)
		}
		fallthrough
	case "null":
		fallthrough
	default:
		return nullAPI{}, nil
	}
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
			"endpoint": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Description: "Endpoint for Fleet.",
			},
			"connection_retries": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  12,
			},
			"etcd_key_prefix": &schema.Schema{
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
	retries := d.Get("connection_retries").(int)
	driver := d.Get("driver").(string)
	endpoint := d.Get("endpoint").(string)
	etcKeyPrefix := d.Get("etcd_key_prefix").(string)
	return getAPI(driver, endpoint, retries, etcKeyPrefix)
}
