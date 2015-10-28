package main

import (
	"errors"
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

func TestGetAPI(test *testing.T) {
	// when the address is an empty string, we get a nullAPI
	var api client.API

	api, err := getAPI("nothing", "non-http-endpoint", "", 1, "")

	if err != nil {
		test.Fatal(err)
	}

	switch api.(type) {
	case nullAPI:
		// pass!
	default:
		test.Errorf("didn't get nullAPI, got %s instead", api)
	}
}
