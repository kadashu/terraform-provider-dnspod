package dnspod

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/cofyc/terraform-provider-dnspod/dnspod/client"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

var (
	recordIDRegex = regexp.MustCompile(`^\d+:\d+$`)
)

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
				// note that this field is type string in the DNSPod API
				Type:         schema.TypeInt,
				ValidateFunc: validation.IntBetween(1, 20),
				Optional:     true, // required if the Type is MX
			},
			"ttl": {
				// note that this field is type string in the DNSPod API
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
			"remark": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func prepareRecordForCreateAndModify(d *schema.ResourceData, record *client.Record, create bool) error {
	var err error
	if create {
		record.DomainId = d.Get("domain_id").(string)
	} else {
		record.DomainId, record.RecordId, err = splitId(d.Id())
		if err != nil {
			return err
		}
	}

	record.SubDomain = d.Get("sub_domain").(string)
	record.RecordType = d.Get("record_type").(string)
	record.RecordLine = d.Get("record_line").(string)
	record.Value = d.Get("value").(string)

	// mx
	if mx, ok := d.GetOk("mx"); ok {
		if record.RecordType != "MX" {
			return fmt.Errorf("mx is not expected when the record type is not MX (type: %s)", record.RecordType)
		}
		record.Mx = strconv.Itoa(mx.(int))
	} else {
		if record.RecordType == "MX" {
			return fmt.Errorf("mx is recorduired when the record type is MX")
		}
	}

	// ttl
	if ttl, ok := d.GetOk("ttl"); ok {
		record.Ttl = strconv.Itoa(ttl.(int))
	} else {
		return fmt.Errorf("ttl is missing")
	}

	// weight
	if weight, ok := d.GetOk("weight"); ok {
		record.Weight = intPtr(weight.(int))
	}

	// status
	record.Status = d.Get("status").(string)

	return nil
}

// https://docs.dnspod.cn/api/5f562a8be75cf42d25bf6886/
func updateRecordRemark(conn *client.Client, domainID, recordID, remark string) error {
	req := client.RecordRemarkRequest{
		DomainId: domainID,
		RecordId: recordID,
		Remark:   remark,
	}
	var resp client.RecordRemarkResponse
	log.Printf("[DEBUG] Record.Remark Request: %+v", req)
	err := conn.Call("Record.Remark", &req, &resp)
	log.Printf("[DEBUG] Record.Remark Response: %+v, Error: %v", resp, err)
	if err != nil {
		return err
	}
	return nil
}

func resourceRecordCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)

	req := client.RecordCreateRequest{}

	if err := prepareRecordForCreateAndModify(d, (*client.Record)(&req), true); err != nil {
		return err
	}

	var resp client.RecordCreateResponse
	log.Printf("[DEBUG] Record.Create Request: %+v", req)
	err := conn.Call("Record.Create", &req, &resp)
	log.Printf("[DEBUG] Record.Create Response: %+v, Error: %v", resp, err)
	if err != nil {
		return err
	}

	id := (*resp.Record.Id).(string)
	d.SetId(req.DomainId + ":" + id)

	remark := d.Get("remark").(string)
	if remark != "" {
		err = updateRecordRemark(conn, req.DomainId, id, remark)
		if err != nil {
			return err
		}
	}

	return nil
}

func resourceRecordUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)

	var err error
	req := client.RecordModifyRequest{}

	if err := prepareRecordForCreateAndModify(d, (*client.Record)(&req), false); err != nil {
		return err
	}

	var resp client.RecordModifyResponse
	log.Printf("[DEBUG] Record.Modify Request: %+v", req)
	err = conn.Call("Record.Modify", &req, &resp)
	log.Printf("[DEBUG] Record.Modify Response: %+v, Error: %v", resp, err)
	if err != nil {
		if bsce, ok := err.(*client.BadStatusCodeError); ok && (bsce.Code == "6" || bsce.Code == "8") {
			// 6 域名ID错误
			// 8 记录ID错误
			d.SetId("")
			return nil
		}

		return err
	}

	if d.HasChange("remark") {
		remark := d.Get("remark").(string)
		err = updateRecordRemark(conn, req.DomainId, req.RecordId, remark)
		if err != nil {
			return err
		}
	}

	return nil
}

func resourceRecordRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*client.Client)

	domainId, recordId, err := splitId(d.Id())
	if err != nil {
		return err
	}
	req := client.RecordInfoRequest{RecordId: recordId, DomainId: domainId}
	var resp client.RecordInfoResponse
	log.Printf("[DEBUG] Record.Info Request: %+v", req)
	err = conn.Call("Record.Info", &req, &resp)
	log.Printf("[DEBUG] Record.Info Response: %+v, Error: %v", resp, err)
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

	// mx
	if resp.Record.RecordType == "MX" {
		// set mx if the type is "MX"
		mx, err := strconv.Atoi(resp.Record.Mx)
		if err != nil {
			return err
		}
		d.Set("mx", mx)
	}

	// ttl
	ttl, err := strconv.Atoi(resp.Record.Ttl)
	if err != nil {
		return err
	}
	d.Set("ttl", ttl)

	if resp.Record.Weight != nil {
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

	d.Set("remark", resp.Record.Remark)

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
	log.Printf("[DEBUG] Record.Remove Request: %+v", req)
	err = conn.Call("Record.Remove", &req, &resp)
	log.Printf("[DEBUG] Record.Remove Response: %+v, Error: %v", resp, err)
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

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}
