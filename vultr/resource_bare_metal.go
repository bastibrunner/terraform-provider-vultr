package vultr

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/JamesClonk/vultr/lib"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceBareMetal() *schema.Resource {
	return &schema.Resource{
		Create: resourceBareMetalCreate,
		Read:   resourceBareMetalRead,
		Update: resourceBareMetalUpdate,
		Delete: resourceBareMetalDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"application_id": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"cpus": {
				Type:     schema.TypeInt,
				Computed: true,
			},

			"default_password": {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},

			"disk": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"hostname": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"ipv4_address": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"ipv6": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"ipv6_address": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"os_id": {
				Type:     schema.TypeInt,
				Required: true,
			},

			"plan_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},

			"ram": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"region_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},

			"snapshot_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"startup_script_id": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},

			"ssh_key_ids": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tag": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceBareMetalCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)
	options := &lib.BareMetalServerOptions{
		AppID:    d.Get("application_id").(string),
		Hostname: d.Get("hostname").(string),
		IPV6:     d.Get("ipv6").(bool),
		Script:   d.Get("startup_script_id").(int),
		Snapshot: d.Get("snapshot_id").(string),
		Tag:      d.Get("tag").(string),
	}

	name := d.Get("name").(string)
	osID := d.Get("os_id").(int)
	planID := d.Get("plan_id").(int)
	regionID := d.Get("region_id").(int)

	keyIDs := make([]string, d.Get("ssh_key_ids.#").(int))
	for i, id := range d.Get("ssh_key_ids").([]interface{}) {
		keyIDs[i] = id.(string)
	}
	options.SSHKey = strings.Join(keyIDs, ",")

	log.Printf("[INFO] Creating new bare metal instance")
	instance, err := client.CreateBareMetalServer(name, regionID, planID, osID, options)
	if err != nil {
		return fmt.Errorf("Error creating bare metal instance: %v", err)
	}
	d.SetId(instance.ID)

	if _, err := waitForResourceState(d, meta, "bare metal instance", "status", resourceBareMetalRead, "active", []string{"pending"}); err != nil {
		return err
	}
	return resourceBareMetalRead(d, meta)
}

func resourceBareMetalRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	instance, err := client.GetBareMetalServer(d.Id())
	if err != nil {
		if err.Error() == "Invalid server." {
			log.Printf("[WARN] Removing bare metal instance (%s) because it is gone", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error getting bare metal instance (%s): %v", d.Id(), err)
	}

	d.Set("application_id", instance.AppID)
	d.Set("cpus", instance.CPUs)
	d.Set("default_password", instance.DefaultPassword)
	d.Set("disk", instance.Disk)
	d.Set("ipv4_address", instance.MainIP)
	d.Set("name", instance.Name)
	osID, err := strconv.Atoi(instance.OSID)
	if err != nil {
		return fmt.Errorf("OS ID must be an integer: %v", err)
	}
	d.Set("os_id", osID)
	d.Set("plan_id", instance.PlanID)
	d.Set("ram", instance.RAM)
	d.Set("region_id", instance.RegionID)
	d.Set("status", instance.Status)
	d.Set("tag", instance.Tag)

	var ipv6s []string
	for _, net := range instance.V6Networks {
		ipv6s = append(ipv6s, net.MainIP)
	}
	d.Set("ipv6_address", ipv6s)

	// Initialize the connection information.
	d.SetConnInfo(map[string]string{
		"host":     instance.MainIP,
		"password": instance.DefaultPassword,
		"type":     "ssh",
		"user":     "root",
	})

	return nil
}

func resourceBareMetalUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	d.Partial(true)

	if d.HasChange("application_id") {
		log.Printf("[INFO] Updating bare metal instance (%s) application", d.Id())
		old, new := d.GetChange("application_id")
		if err := client.ChangeApplicationofBareMetalServer(d.Id(), new.(string)); err != nil {
			return fmt.Errorf("Error changing application of instance (%s) to %q: %v", d.Id(), new.(string), err)
		}
		if _, err := waitForResourceState(d, meta, "bare metal instance", "application_id", resourceBareMetalRead, new.(string), []string{"", old.(string)}); err != nil {
			return err
		}
		d.SetPartial("application_id")
	}

	if d.HasChange("name") {
		log.Printf("[INFO] Updating bare metal instance (%s) name", d.Id())
		old, new := d.GetChange("name")
		if err := client.RenameBareMetalServer(d.Id(), new.(string)); err != nil {
			return fmt.Errorf("Error renaming bare metal instance (%s) to %q: %v", d.Id(), new.(string), err)
		}
		if _, err := waitForResourceState(d, meta, "bare metal instance", "name", resourceBareMetalRead, new.(string), []string{"", old.(string)}); err != nil {
			return err
		}
		d.SetPartial("name")
	}

	if d.HasChange("os_id") {
		log.Printf("[INFO] Updating bare metal instance (%s) OS", d.Id())
		old, new := d.GetChange("os_id")
		if err := client.ChangeOSofBareMetalServer(d.Id(), new.(int)); err != nil {
			return fmt.Errorf("Error changing OS of bare metal instance (%s) to %d: %v", d.Id(), new.(int), err)
		}
		if _, err := waitForResourceState(d, meta, "bare metal instance", "os_id", resourceBareMetalRead, strconv.FormatInt(int64(new.(int)), 10), []string{"", strconv.FormatInt(int64(old.(int)), 10)}); err != nil {
			return err
		}
		d.SetPartial("os_id")
	}

	if d.HasChange("tag") {
		log.Printf("[INFO] Updating bare metal instance (%s) tag", d.Id())
		old, new := d.GetChange("tag")
		if err := client.TagBareMetalServer(d.Id(), new.(string)); err != nil {
			return fmt.Errorf("Error tagging bare metal instance (%s) with %q: %v", d.Id(), new.(string), err)
		}
		if _, err := waitForResourceState(d, meta, "bare metal instance", "tag", resourceBareMetalRead, new.(string), []string{"", old.(string)}); err != nil {
			return err
		}
		d.SetPartial("tag")
	}

	d.Partial(false)

	return resourceBareMetalRead(d, meta)
}

func resourceBareMetalDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Client)

	log.Printf("[INFO] Destroying bare metal instance (%s)", d.Id())

	if err := client.DeleteBareMetalServer(d.Id()); err != nil {
		return fmt.Errorf("Error destroying bare metal instance (%s): %v", d.Id(), err)
	}

	return nil
}
