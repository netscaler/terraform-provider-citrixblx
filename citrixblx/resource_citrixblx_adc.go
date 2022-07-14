package citrixblx

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
	"net"
)

func resourceCitrixBLXADC() *schema.Resource {
	return &schema.Resource{
		Create: resourceBLXCreate,
		Read:   resourceBLXRead,
		Update: resourceBLXUpdate,
		Delete: resourceBLXDelete,

		Schema: map[string]*schema.Schema{
			"source": {
				Type:     schema.TypeString,
				Required: true,
			},
			"host": {
				Type:     schema.TypeMap,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ipaddress": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"username": {
							Type:     schema.TypeString,
							Required: true,
						},
						"password": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"keyfile": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"port": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"ssh_hostkey_check": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"config": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ipaddress": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"total_hugepage_mem": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"host_ipaddress": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"blx_managed_host": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"nsdrvd": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"interfaces": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"mgmt_http_port": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"mgmt_https_port": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"mgmt_ssh_port": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"worker_processes": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"cpu_yield": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"default_gateway": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"password": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"cli_cmd": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"mlx_ofed": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"mlx_tools": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"local_license": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

// Create the BLX struct from Resource Schema
func getBlxFromSchema(d *schema.ResourceData, function string) (blx, error) {
	source := d.Get("source").(string)

	host := getHostInfo(d.Get("host").(map[string]interface{}))

	config := getConfigInfo(d.Get("config").(map[string]interface{}))

	cliCmdList := make([]string, 0)
	if d.Get("cli_cmd") != nil {
		tmpList := d.Get("cli_cmd").([]interface{})
		for _, cmd := range tmpList {
			cliCmdList = append(cliCmdList, cmd.(string))
		}
	}

	licenseList := make([]string, 0)
	if d.Get("local_license") != nil {
		tmpList := d.Get("local_license").([]interface{})
		for _, i := range tmpList {
			licenseList = append(licenseList, i.(string))
		}
	}

	mlx := make(map[string]string)
	mlx["ofed"] = d.Get("mlx_ofed").(string)
	mlx["tools"] = d.Get("mlx_tools").(string)

	password := d.Get("password").(string)

	id := host["ipaddress"]
	if config["ipaddress"] != "" {
		id = ""
		addr := net.ParseIP(config["ipaddress"])
		if addr == nil {
			addr, _, err := net.ParseCIDR(config["ipaddress"])
			if err == nil {
				id = fmt.Sprintf("%v", addr)
			}
		} else {
			id = config["ipaddress"]
		}
	}

	b := blx{
		id:          id,
		mlx:         mlx,
		source:      source,
		host:        host,
		config:      config,
		cliCmd:      cliCmdList,
		password:    password,
		nsSession:   nil,
		hostSession: nil,
		licenseList: licenseList,
	}
	err := validateBLX(b)
	if err != nil {
		return b, err
	}

	if function != "create" {
		b.nsSession, err = nsConnect(&b)
		if err != nil {
			var err1 error
			b.hostSession, err1 = hostConnect(host)
			if err1 != nil {
				return b, err
			}
			err = nil
		}
	} else {
		b.hostSession, err = hostConnect(host)
	}

	if err != nil {
		return b, err
	}

	if b.hostSession != nil {
		initBLXHost(&b)
	}

	if err != nil {
		return b, err
	}

	return b, nil
}

func resourceBLXCreate(d *schema.ResourceData, m interface{}) error {
	log.Printf("[DEBUG]  citrixblx-provider: In BLX Create Function")

	b, err := getBlxFromSchema(d, "create")
	if err != nil {
		return err
	}

	err = setupBLX(&b)
	if err != nil {
		log.Printf("[ERROR] citrixblx-provider: Unable to Install BLX")
		return err
	}
	d.SetId(b.id)

	log.Printf("[DEBUG]  citrixblx-provider: BLX Create SUCCESS")
	return nil
}

func resourceBLXRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceBLXUpdate(d *schema.ResourceData, m interface{}) error {
	log.Printf("[DEBUG]  citrixblx-provider: In BLX Update Function")

	b, err := getBlxFromSchema(d, "update")
	if err != nil {
		d.SetId("")
		return err
	}

	if d.HasChange("source") {
		err := installBLX(&b)
		if err != nil {
			d.SetId("")
			log.Printf("[ERROR] citrixblx-provider: Unable to Install BLX in Update")
			return err
		}
	}

	// all updates besides install
	// handled by initBLX
	err = initBLX(&b)
	if err != nil {
		log.Printf("[ERROR] citrixblx-provider: Unable to update BLX with new parameters")
		return err
	}

	log.Printf("[INFO]  citrixblx-provider: BLX Update Succeeded")
	return nil
}

func resourceBLXDelete(d *schema.ResourceData, m interface{}) error {
	log.Printf("[DEBUG]  citrixblx-provider: In BLX Delete Function")

	b, err := getBlxFromSchema(d, "delete")
	if err != nil {
		return err
	}

	err = destroyBLX(&b)
	if err != nil {
		log.Printf("[ERROR] citrixblx-provider: Unable to destroy BLX")
		return err
	}

	d.SetId("")

	log.Printf("[DEBUG]  citrixblx-provider: BLX Destroy Succeeded")
	return nil
}
