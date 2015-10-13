package main

import (
	"testing"

	"github.com/coreos/fleet/client"
)

func TestNullAPI(test *testing.T) {
	var api client.API = new(nullAPI)

	// all the methods return errors, all the time.

	if _, err := api.Machines(); err == nil {
		test.Error("Machines() did not return an error")
	}

	if _, err := api.Unit("foo"); err == nil {
		test.Error("Unit() did not return an error")
	}

	if _, err := api.Units(); err == nil {
		test.Error("Units() did not return an error")
	}

	if _, err := api.UnitStates(); err == nil {
		test.Error("UnitStates() did not return an error")
	}

	if err := api.SetUnitTargetState("foo", "bar"); err == nil {
		test.Error("SetUnitTargetState() did not return an error")
	}

	if err := api.CreateUnit(nil); err == nil {
		test.Error("CreateUnit() did not return an error")
	}

	if err := api.DestroyUnit("foo"); err == nil {
		test.Error("DestroyUnit() did not return an error")
	}
}
