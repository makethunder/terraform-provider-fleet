package main

import (
	"errors"
	"testing"
	"strings"

	"github.com/coreos/fleet/client"
	"github.com/hashicorp/terraform/helper/schema"
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

func TestProvider(test *testing.T) {
	provider := Provider().(*schema.Provider)
	if err := provider.InternalValidate(); err != nil {
		test.Fatalf("err: %s", err)
	}
}

func TestRetry(test *testing.T) {
	// if the underlying function times out enough, the error is passed
	// through.
	failAlways := func() (interface{}, error) {
		return nil, errors.New("timed out")
	}

	_, err := retry(failAlways, 2)

	if err == nil || err.Error() != "timed out" {
		test.Errorf("did not receive failure from underlying function, got %s instead", err)
	}

	// generate a function that fails the first time with a specified message
	makeFailOnce := func(errorMessage string) func() (interface{}, error) {
		retries := 0
		return func() (interface{}, error) {
			if retries > 0 {
				return nil, nil
			}
			retries++
			return nil, errors.New(errorMessage)
		}
	}

	// if the error contains "timed out", then retry
	_, err = retry(makeFailOnce("this one time, at band camp, I timed out"), 2)
	if err != nil {
		test.Errorf("apparently did not retry: got failure %s", err)
	}

	// if the error does not contain "timed out", don't retry
	_, err = retry(makeFailOnce("some other problem"), 2)
	if err == nil {
		test.Error("retried for non-timeout error")
	}
}

func TestGetRegistryAPI(test *testing.T) {
	// when the address is an empty string, we get a nullAPI
	var api client.API

	api, err := getAPI(Conf{
		ClientDriver : "api",
		Endpoint : "",
		EtcdKeyPrefix : "",
		KeyFile : "",
		CertFile : "",
		CAFile : "",
		Tunnel : "",
		KnownHostsFile : "",
		SSHUserName : "",

		StrictHostKeyChecking : false,

		SSHTimeout : 1.0,
		RequestTimeout : 1.0,
	})

	if err != nil {
		test.Fatal(err)
	}

	switch api.(type) {
	case client.RegistryClient:
		// pass!
	default:
		test.Errorf("didn't get nullAPI, got %s instead", api)
	}
}

func TestGetHTTPAPI(test *testing.T) {
	// when the address is an empty string, we get a nullAPI
	var api client.API

	api, err := getAPI(Conf{
		ClientDriver : "api",
		Endpoint : "",
		EtcdKeyPrefix : "",
		KeyFile : "",
		CertFile : "",
		CAFile : "",
		Tunnel : "",
		KnownHostsFile : "",
		SSHUserName : "",

		StrictHostKeyChecking : false,

		SSHTimeout : 1.0,
		RequestTimeout : 1.0,
	})

	if err != nil {
		test.Fatal(err)
	}

	switch api.(type) {
	case client.HTTPClient:
		// pass!
	default:
		test.Errorf("didn't get nullAPI, got %s instead", api)
	}
}
