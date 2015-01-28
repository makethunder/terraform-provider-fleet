package fleet

import (
	"errors"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

type nullAPI struct {
	/*
		I implement client.API, but all my methods return errors.

		Use case: in one Terraform configuration you are creating a CoreOS
		cluster, and then provisioning some units on Fleet. The fleet provider
		configuration will necessarilly depend on the IP address of one of the
		CoreOS boxes, but that isn't known until its actually created.

		Terraform runs in two phases: first it plans, and then it applies. This
		provider will use a nullAPI in the planning stage when the IP address
		is not known. In this case, none of the API methods should ever be
		called (which is why they all return errors), because it's not
		necessary to use the API to plan: since nothing exists yet, all
		resources must be created.
	*/
}

func (nullAPI) Machines() ([]machine.MachineState, error) {
	return nil, errors.New("can't list machines: not connected to a real Fleet API")
}

func (nullAPI) Unit(string) (*schema.Unit, error) {
	return nil, errors.New("can't list unit: not connected to a real Fleet API")
}

func (nullAPI) Units() ([]*schema.Unit, error) {
	return nil, errors.New("can't list units: not connected to a real Fleet API")
}

func (nullAPI) UnitStates() ([]*schema.UnitState, error) {
	return nil, errors.New("can't list unit states: not connected to a real Fleet API")
}

func (nullAPI) SetUnitTargetState(string, string) error {
	return errors.New("can't set unit target state: not connected to a real Fleet API")
}

func (nullAPI) CreateUnit(*schema.Unit) error {
	return errors.New("can't create unit: not connected to a real Fleet API")
}

func (nullAPI) DestroyUnit(string) error {
	return errors.New("can't destroy unit: not connected to a real Fleet API")
}
