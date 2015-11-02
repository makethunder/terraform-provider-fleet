package main

import (
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/hashicorp/terraform/helper/schema"
)

func TestProvider(test *testing.T) {
	provider := Provider().(*schema.Provider)
	if err := provider.InternalValidate(); err != nil {
		test.Fatalf("err: %s", err)
	}
}

func TestGetRegistryAPI(test *testing.T) {
	// when the address is an empty string, we get a nullAPI
	var api client.API

	api, err := getAPI(Conf{
		ClientDriver : "etcd",

		StrictHostKeyChecking : false,

		SSHTimeout : 1.0,
		RequestTimeout : 1.0,
	})

	if err != nil {
		test.Fatal(err)
	}

	switch api.(type) {
	case *client.RegistryClient:
		// pass!
	default:
		test.Errorf("didn't get Registry API, got %s instead", api)
	}
}

func TestGetHTTPAPI(test *testing.T) {
	// when the address is an empty string, we get a nullAPI
	var api client.API

	api, err := getAPI(Conf{
		ClientDriver : "api",
		Endpoint : "http://falselocation.test:8080",

		StrictHostKeyChecking : false,

		SSHTimeout : 1.0,
		RequestTimeout : 1.0,
	})

	if err != nil {
		test.Fatal(err)
	}

	switch api.(type) {
	case *client.HTTPClient:
		// pass!
	default:
		test.Errorf("didn't get HTTP API, got %s instead", api)
	}
}
