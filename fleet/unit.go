package fleet

import (
	"log"

	"github.com/coreos/fleet/client"
	fleet "github.com/coreos/fleet/schema"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceUnit() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"state": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "one of \"inactive\", \"loaded\", or \"launched\"",
			},

			"machineid": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"section": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem:     resourceUnitSection(),
			},
		},

		Create: resourceUnitCreate,
		Read:   resourceUnitRead,
		Update: resourceUnitUpdate,
		Delete: resourceUnitDelete,
	}
}

func resourceUnitSection() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"option": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem:     resourceUnitOption(),
			},
		},
	}
}

func resourceUnitOption() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"value": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceUnitCreate(d *schema.ResourceData, meta interface{}) error {
	api := meta.(client.API)
	var options []*fleet.UnitOption

	for _, s := range d.Get("section").([]interface{}) {
		section := s.(map[string]interface{})
		for _, o := range section["option"].([]interface{}) {
			option := o.(map[string]interface{})
			options = append(options, &fleet.UnitOption{
				Section: section["name"].(string),
				Name:    option["name"].(string),
				Value:   option["value"].(string),
			})
		}
	}

	unit := &fleet.Unit{
		Name:         d.Get("name").(string),
		DesiredState: d.Get("state").(string),
		Options:      options,
	}

	err := api.CreateUnit(unit)
	if err != nil {
		return err
	}

	d.SetId(d.Get("name").(string))
	return resourceUnitRead(d, meta)
}

func resourceUnitRead(d *schema.ResourceData, meta interface{}) error {
	log.Println("[DEBUG] reading fleet unit", d.Id())
	api := meta.(client.API)
	unit, err := api.Unit(d.Id())
	if err != nil {
		return err
	}
	if unit == nil {
		log.Printf("[DEBUG] api.Unit(%v) returned nil. Unit is no more", d.Id())
		d.SetId("")
		return nil
	}

	if err := d.Set("state", unit.CurrentState); err != nil {
		return err
	}
	if err := d.Set("machineid", unit.MachineID); err != nil {
		return err
	}
	if err := d.Set("name", unit.Name); err != nil {
		return err
	}
	return nil
}

func resourceUnitUpdate(d *schema.ResourceData, meta interface{}) error {
	api := meta.(client.API)
	err := api.SetUnitTargetState(d.Id(), d.Get("state").(string))
	if err != nil {
		return err
	}
	return resourceUnitRead(d, meta)
}

func resourceUnitDelete(d *schema.ResourceData, meta interface{}) error {
	api := meta.(client.API)
	return api.DestroyUnit(d.Id())
}
