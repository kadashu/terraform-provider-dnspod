package dnspod

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/cofyc/terraform-provider-dnspod/dnspod/client"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

var (
	recordIDRegex = regexp.MustCompile(`^\d+:\d+$`)
)

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

func resourceRecord() *schema.Resource {
	return &schema.Resource{
		Create: resourceRecordCreate,
		Read:   resourceRecordRead,
		Update: resourceRecordUpdate,
		Delete: resourceRecordDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"domain_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"sub_domain": {
				Type:     schema.TypeString,
				Required: true,
			},
			"record_type": {
				Type:     schema.TypeString,
				Required: true,
			},
			"record_line": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "默认",
			},
			"value": {
				Type:     schema.TypeString,
				Required: true,
			},
			"mx": {
				Type:         schema.TypeInt,
				ValidateFunc: validation.IntBetween(1, 20),
				Optional:     true, // required if the Type is MX
			},
			"ttl": {
				Type:         schema.TypeInt,
				ValidateFunc: validation.IntBetween(1, 604800), // 不同等级域名最小值不同
				Optional:     true,
				Default:      600,
			},
			"weight": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntBetween(0, 100),
			},
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"enable", "disable"}, false),
				Default:      "enable",
			},
		},
	}
}

func resourceRecordCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)

	req := client.RecordCreateRequest{}
	req.DomainId = d.Get("domain_id").(string)
	req.SubDomain = d.Get("sub_domain").(string)
	req.RecordType = d.Get("record_type").(string)
	req.RecordLine = d.Get("record_line").(string)
	req.Value = d.Get("value").(string)
	req.Mx = d.Get("mx").(string)
	req.Ttl = d.Get("ttl").(string)
	if weight, ok := d.GetOk("weight"); ok {
		req.Weight = intPtr(weight.(int))
	}
	req.Status = d.Get("status").(string)

	var resp client.RecordCreateResponse
	err := conn.Call("Record.Create", &req, &resp)
	if err != nil {
		return err
	}

	id := (*resp.Record.Id).(string)
	d.SetId(req.DomainId + ":" + id)

	return nil
}

func resourceRecordUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)

	var err error
	req := client.RecordModifyRequest{}
	req.DomainId, req.RecordId, err = splitId(d.Id())
	if err != nil {
		return err
	}
	req.SubDomain = d.Get("sub_domain").(string)
	req.RecordType = d.Get("record_type").(string)
	req.RecordLine = d.Get("record_line").(string)
	req.Value = d.Get("value").(string)
	req.Mx = d.Get("mx").(string)
	req.Ttl = d.Get("ttl").(string)
	if weight, ok := d.GetOk("weight"); ok {
		req.Weight = intPtr(weight.(int))
	}
	req.Status = d.Get("status").(string)

	var resp client.RecordModifyResponse
	err = conn.Call("Record.Modify", &req, &resp)
	if err != nil {
		if bsce, ok := err.(*client.BadStatusCodeError); ok && (bsce.Code == "6" || bsce.Code == "8") {
			// 6 域名ID错误
			// 8 记录ID错误
			d.SetId("")
			return nil
		}

		return err
	}

	return nil
}

func resourceRecordRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)
	log.Printf("[DEBUG] reading id %s", d.Id())

	domainId, recordId, err := splitId(d.Id())
	if err != nil {
		return err
	}
	req := client.RecordInfoRequest{RecordId: recordId, DomainId: domainId}
	var resp client.RecordInfoResponse
	err = conn.Call("Record.Info", &req, &resp)
	if err != nil {
		if bsce, ok := err.(*client.BadStatusCodeError); ok && (bsce.Code == "6" || bsce.Code == "8") {
			// 6 域名ID错误
			// 8 记录ID错误
			d.SetId("")
			return nil
		}

		return err
	}

	d.Set("domain_id", domainId)
	d.Set("sub_domain", resp.Record.SubDomain)
	d.Set("record_type", resp.Record.RecordType)
	d.Set("record_line", resp.Record.RecordLine)
	d.Set("value", resp.Record.Value)
	d.Set("mx", resp.Record.Mx)
	d.Set("ttl", resp.Record.Ttl)
	log.Printf("[DEBUG] %+v", resp.Record)
	if resp.Record.Weight != nil {
		log.Printf("[DEBUG] weight %d", *resp.Record.Weight)
		d.Set("weight", *resp.Record.Weight)
	}

	var status string
	if resp.Record.Enabled == "1" {
		status = "enable"
	} else if resp.Record.Enabled == "0" {
		status = "disable"
	} else {
		return fmt.Errorf("unexpect Enable field (0 or 1), got %s", resp.Record.Enabled)
	}
	d.Set("status", status)

	return nil
}

func resourceRecordDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)

	domainId, recordId, err := splitId(d.Id())
	if err != nil {
		return err
	}
	var resp client.RecordRemoveResponse
	req := client.RecordRemoveRequest{DomainId: domainId, RecordId: recordId}
	err = conn.Call("Record.Remove", &req, &resp)
	if err != nil {
		if bsce, ok := err.(*client.BadStatusCodeError); !ok || (bsce.Code != "6" && bsce.Code != "8") {
			return err
		}
	}

	d.SetId("")

	return nil
}

func splitId(id string) (string, string, error) {
	if ok := recordIDRegex.MatchString(id); !ok {
		return "", "", fmt.Errorf("expects state id to be in the format <domain_id>:<record_id>, got '%s", id)
	}
	parts := strings.SplitN(id, ":", 2)
	return parts[0], parts[1], nil
}
