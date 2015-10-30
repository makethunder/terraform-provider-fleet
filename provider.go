package main

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"fmt"
	"errors"
	"os"

	"github.com/coreos/fleet/log"
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

// Conf - Config for fleet
type Conf struct {
	ClientDriver    		string
	ExperimentalAPI 		bool
	Endpoint        		string
	RequestTimeout  		float64

	KeyFile  				string
	CertFile 				string
	CAFile   				string

	Tunnel                	string
	KnownHostsFile        	string
	StrictHostKeyChecking 	bool
	SSHTimeout            	float64
	SSHUserName           	string

	EtcdKeyPrefix 			string
}

const (
	clientDriverAPI = "api"
	clientDriverEtcd = "etcd"
	defaultEndpoint = "unix:///var/run/fleet.sock"
)

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

func stderr(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, maybeAddNewline(format), args...)
}

func stdout(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, maybeAddNewline(format), args...)
}

func maybeAddNewline(s string) string {
	if !strings.HasSuffix(s, "\n") {
		s = s + "\n"
	}
	return s
}

func getHTTPClient(conf Conf) (client.API, error) {
	endpoints := strings.Split(conf.Endpoint, ",")
	if len(endpoints) > 1 {
		log.Warningf("multiple endpoints provided but only the first (%s) is used", endpoints[0])
	}

	ep, err := url.Parse(endpoints[0])
	if err != nil {
		return nil, err
	}

	if len(ep.Scheme) == 0 {
		return nil, errors.New("URL scheme undefined")
	}

	tun := getTunnelFlag(conf)
	tunneling := tun != ""

	dialUnix := ep.Scheme == "unix" || ep.Scheme == "file"

	tunnelFunc := net.Dial
	if tunneling {
		sshClient, err := ssh.NewSSHClient(conf.SSHUserName, tun, getChecker(conf), true, getSSHTimeoutFlag(conf))
		if err != nil {
			return nil, fmt.Errorf("failed initializing SSH client: %v", err)
		}

		if dialUnix {
			tgt := ep.Path
			tunnelFunc = func(string, string) (net.Conn, error) {
				log.Debugf("Establishing remote fleetctl proxy to %s", tgt)
				cmd := fmt.Sprintf(`fleetctl fd-forward %s`, tgt)
				return ssh.DialCommand(sshClient, cmd)
			}
		} else {
			tunnelFunc = sshClient.Dial
		}
	}

	dialFunc := tunnelFunc
	if dialUnix {
		// This commonly happens if the user misses the leading slash after the scheme.
		// For example, "unix://var/run/fleet.sock" would be parsed as host "var".
		if len(ep.Host) > 0 {
			return nil, fmt.Errorf("unable to connect to host %q with scheme %q", ep.Host, ep.Scheme)
		}

		// The Path field is only used for dialing and should not be used when
		// building any further HTTP requests.
		sockPath := ep.Path
		ep.Path = ""

		// If not tunneling to the unix socket, http.Client will dial it directly.
		// http.Client does not natively support dialing a unix domain socket, so the
		// dial function must be overridden.
		if !tunneling {
			dialFunc = func(string, string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			}
		}

		// http.Client doesn't support the schemes "unix" or "file", but it
		// is safe to use "http" as dialFunc ignores it anyway.
		ep.Scheme = "http"

		// The Host field is not used for dialing, but will be exposed in debug logs.
		ep.Host = "domain-sock"
	}

	tlsConfig, err := pkg.ReadTLSConfigFiles(conf.CAFile, conf.CertFile, conf.KeyFile)
	if err != nil {
		return nil, err
	}

	trans := pkg.LoggingHTTPTransport{
		Transport: http.Transport{
			Dial:            dialFunc,
			TLSClientConfig: tlsConfig,
		},
	}

	hc := http.Client{
		Transport: &trans,
	}

	return client.NewHTTPClient(&hc, *ep)
}

func getRegistryClient(conf Conf) (client.API, error) {
	var dial func(string, string) (net.Conn, error)
	tun := getTunnelFlag(conf)
	if tun != "" {
		sshClient, err := ssh.NewSSHClient(conf.SSHUserName, tun, getChecker(conf), false, getSSHTimeoutFlag(conf))
		if err != nil {
			return nil, fmt.Errorf("failed initializing SSH client: %v", err)
		}

		dial = func(network, addr string) (net.Conn, error) {
			tcpaddr, err := net.ResolveTCPAddr(network, addr)
			if err != nil {
				return nil, err
			}
			return sshClient.DialTCP(network, nil, tcpaddr)
		}
	}

	tlsConfig, err := pkg.ReadTLSConfigFiles(conf.CAFile, conf.CertFile, conf.KeyFile)
	if err != nil {
		return nil, err
	}

	trans := &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
	}

	eCfg := etcd.Config{
		Endpoints: strings.Split(conf.Endpoint, ","),
		Transport: trans,
	}

	eClient, err := etcd.New(eCfg)
	if err != nil {
		return nil, err
	}

	keysAPI := etcd.NewKeysAPI(eClient)
	reg := registry.NewEtcdRegistry(keysAPI, conf.EtcdKeyPrefix, getRequestTimeoutFlag(conf))

	if msg, ok := checkVersion(reg); !ok {
		stderr(msg)
	}

	return &client.RegistryClient{Registry: reg}, nil
}

func getTunnelFlag(conf Conf) string {
	tun := conf.Tunnel
	if tun != "" && !strings.Contains(tun, ":") {
		tun += ":22"
	}
	return tun
}

func getSSHTimeoutFlag(conf Conf) time.Duration {
	return time.Duration(conf.SSHTimeout*1000) * time.Millisecond
}

func getRequestTimeoutFlag(conf Conf) time.Duration {
	return time.Duration(conf.RequestTimeout*1000) * time.Millisecond
}

// getChecker creates and returns a HostKeyChecker, or nil if any error is encountered
func getChecker(conf Conf) *ssh.HostKeyChecker {
	if !conf.StrictHostKeyChecking {
		return nil
	}

	keyFile := ssh.NewHostKeyFile(conf.KnownHostsFile)
	return ssh.NewHostKeyChecker(keyFile)
}

// getAPI returns an API to Fleet.
func getAPI(conf Conf) (client.API, error) {
	switch strings.ToLower(conf.ClientDriver) {
	case clientDriverAPI:
		return getHTTPClient(conf)
	case clientDriverEtcd:
		return getRegistryClient(conf)
	}

	return nil, fmt.Errorf("unrecognized driver %q", conf.ClientDriver)
}

// Provider returns the ResourceProvider implemented by this package. Serve
// this with the Terraform plugin helper and you are golden.
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
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
		},
		ResourcesMap: map[string]*schema.Resource{
			"fleet_unit": resourceUnit(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	return getAPI(Conf{
		ClientDriver : d.Get("driver").(string),
		Endpoint : d.Get("endpoint").(string),
		EtcdKeyPrefix : d.Get("etcd-key-prefix").(string),
		KeyFile : d.Get("key-file").(string),
		CertFile : d.Get("cert-file").(string),
		CAFile : d.Get("ca-file").(string),
		Tunnel : d.Get("tunnel").(string),
		KnownHostsFile : d.Get("known-hosts-file").(string),
		SSHUserName : d.Get("ssh-username").(string),

		StrictHostKeyChecking : d.Get("strict-host-key-checking").(bool),

		SSHTimeout : d.Get("ssh-timeout").(float64),
		RequestTimeout : d.Get("request-timeout").(float64),
	})
}
