package gtm

import (
	"context"
	"fmt"
	"strings"
	"time"
	"errors"

	"github.com/akamai/terraform-provider-akamai/v2/pkg/akamai"
	"github.com/akamai/terraform-provider-akamai/v2/pkg/tools"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/client-v1"
	gtm "github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1_4"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Hack for Hashicorp Acceptance Tests
var HashiAcc = false

func resourceGTMv1Domain() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGTMv1DomainCreate,
		ReadContext:   resourceGTMv1DomainRead,
		UpdateContext: resourceGTMv1DomainUpdate,
		DeleteContext: resourceGTMv1DomainDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"contract": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"group": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"wait_on_complete": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateDomainType,
			},
			"comment": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Managed by Terraform",
			},
			"default_unreachable_threshold": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"email_notification_list": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
			"min_pingable_region_fraction": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"default_timeout_penalty": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  25,
			},
			"servermonitor_liveness_count": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"round_robin_prefix": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"servermonitor_load_count": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"ping_interval": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"max_ttl": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"load_imbalance_percentage": {
				Type:     schema.TypeFloat,
				Optional: true,
			},
			"default_health_max": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"map_update_interval": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"max_properties": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"max_resources": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"default_ssl_client_private_key": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"default_error_penalty": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  75,
			},
			"max_test_timeout": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"cname_coalescing_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"default_health_multiplier": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"servermonitor_pool": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"load_feedback": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"min_ttl": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"default_max_unreachable_penalty": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"default_health_threshold": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"min_test_interval": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"ping_packet_size": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"default_ssl_client_certificate": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"end_user_mapping_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
			},
		},
	}
}

// Retrieve optional query args. contractId, groupId [and accountSwitchKey] supported.
func GetQueryArgs(d *schema.ResourceData) (map[string]string, error) {

	qArgs := make(map[string]string)
	contractName, err := tools.GetStringValue("contract", d)
	if err != nil {
		return nil, fmt.Errorf("contract not present in resource data: %v", err.Error())
	}
	contract := strings.TrimPrefix(contractName, "ctr_")
	if contract != "" && len(contract) > 0 {
		qArgs["contractId"] = contract
	}
	groupName, err := tools.GetStringValue("group", d)
	if err != nil {
		return nil, fmt.Errorf("group not present in resource data: %v", err.Error())
	}
	groupId := strings.TrimPrefix(groupName, "grp_")
	if groupId != "" && len(groupId) > 0 {
		qArgs["gid"] = groupId
	}

	return qArgs, nil
}

// Create a new GTM Domain
func resourceGTMv1DomainCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTM", "resourceGTMv1DomainCreate")

	dname, err := tools.GetStringValue("name", d)
	if err != nil {
		logger.Errorf("Domain name not found in ResourceData")
		return diag.FromErr(err)
	}
	logger.Infof("Creating domain [%s]", dname)
	newDom, err := populateNewDomainObject(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	logger.Debugf("Domain: [%v]", newDom)
	var diags diag.Diagnostics
	queryArgs, err := GetQueryArgs(d)
	if err != nil {
		logger.Errorf("Domain Create failed: %s", err.Error())
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Domain Create failed",
			Detail:   err.Error(),
		})
	}
	cStatus, err := newDom.Create(queryArgs)
	if err != nil {
		// Errored. Lets see if special hack
		if !HashiAcc {
			logger.Errorf("Domain Create failed: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Create failed",
				Detail:   err.Error(),
			})
		}
		if _, ok := err.(gtm.CommonError); !ok {
			logger.Errorf("Domain Create failed: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Create failed",
				Detail:   err.Error(),
			})
		}
		origErr, ok := err.(gtm.CommonError).GetItem("err").(client.APIError)
		if !ok {
			logger.Errorf("DomainCreate failed: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Create failed",
				Detail:   err.Error(),
			})
		}
		if origErr.Status == 400 && strings.Contains(origErr.RawBody, "proposed domain name") && strings.Contains(origErr.RawBody, "Domain Validation Error") {
			// Already exists
			logger.Warnf("Domain %s already exists. Ignoring error (Hashicorp).", dname)
		} else {
			logger.Errorf("Error creating Domain [%s]", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Create failed",
				Detail:   err.Error(),
			})
		}
	} else {
		logger.Debugf("Create status: %v", cStatus.Status)
		if cStatus.Status.PropagationStatus == "DENIED" {
			logger.Errorf(cStatus.Status.Message)
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  cStatus.Status.Message,
			})
		}

		waitOnComplete, err := tools.GetBoolValue("wait_on_complete", d)
		if err != nil {
			return diag.FromErr(err)
		}

		if waitOnComplete {
			done, err := waitForCompletion(dname, m)
			if done {
				logger.Infof("Domain Create completed")
			} else {
				if err == nil {
					logger.Infof("Domain Create pending")
				} else {
					logger.Errorf("Domain Create failed [%s]", err.Error())
					return append(diags, diag.Diagnostic{
						Severity: diag.Error,
						Summary:  "Domain Create failed",
						Detail:   err.Error(),
					})
				}
			}
		}
	}
	// Give terraform the ID
	d.SetId(dname)
	return resourceGTMv1DomainRead(ctx, d, m)

}

// Only ever save data from the tf config in the tf state file, to help with
// api issues. See func unmarshalResourceData for more info.
func resourceGTMv1DomainRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTM", "resourceGTMv1DomainRead")

	logger.Debugf("Reading Domain: %s", d.Id())
	var diags diag.Diagnostics
	// retrieve the domain
	dom, err := gtm.GetDomain(d.Id())
	if err != nil {
		logger.Errorf("Domain Read error: %s", err.Error())
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Domain Read error",
			Detail:   err.Error(),
		})
	}
	populateTerraformState(d, dom, m)
	logger.Debugf("READ %v", dom)
	return nil
}

// Update GTM Domain
func resourceGTMv1DomainUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTM", "resourceGTMv1DomainUpdate")

	logger.Debugf("Updating Domain: %s", d.Id())
	var diags diag.Diagnostics
	// Get existing domain
	existDom, err := gtm.GetDomain(d.Id())
	if err != nil {
		logger.Errorf("Domain Update failed: %s", err.Error())
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Update Domain read failed",
			Detail:   err.Error(),
		})
	}
	logger.Debugf("Updating Domain BEFORE: %v", existDom)
	err = populateDomainObject(d, existDom, m)
	if err != nil {
		return diag.FromErr(err)
	}
	logger.Debugf("Updating Domain PROPOSED: %v", existDom)
	//existDom := populateNewDomainObject(d)
	args, err := GetQueryArgs(d)
	if err != nil {
		logger.Errorf("Domain Update failed: %s", err.Error())
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Domain Update error",
			Detail:   err.Error(),
		})
	}
	uStat, err := existDom.Update(args)
	if err != nil {
		logger.Errorf("Domain Update failed: %s", err.Error())
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Domain Update error",
			Detail:   err.Error(),
		})
	}
	logger.Debugf("Update status: %v", uStat)
	if uStat.PropagationStatus == "DENIED" {
		logger.Errorf(uStat.Message)
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  uStat.Message,
		})
	}

	waitOnComplete, err := tools.GetBoolValue("wait_on_complete", d)
	if err != nil {
		return diag.FromErr(err)
	}

	if waitOnComplete {
		done, err := waitForCompletion(d.Id(), m)
		if done {
			logger.Infof("Domain Update completed")
		} else {
			if err == nil {
				logger.Infof("Domain Update pending")
			} else {
				logger.Errorf("Domain Update failed [%s]", err.Error())
				return append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  "domain Update failed",
					Detail:   err.Error(),
				})
			}
		}

	}

	return resourceGTMv1DomainRead(ctx, d, m)

}

// Delete GTM Domain. Admin privileges required in current API version.
func resourceGTMv1DomainDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTM", "resourceGTMv1DomainDelete")

	logger.Debugf("Deleting GTM Domain: %s", d.Id())
	var diags diag.Diagnostics
	// Get existing domain
	existDom, err := gtm.GetDomain(d.Id())
	if err != nil {
		logger.Errorf("Domain Delete failed: %s", err.Error())
		return append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("Invalid domain ID: %s", d.Id()),
			Detail:   err.Error(),
		})
	}
	uStat, err := existDom.Delete()
	if err != nil {
		// Errored. Lets see if special hack
		if !HashiAcc {
			logger.Errorf("Error Domain Delete: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Delete error",
				Detail:   err.Error(),
			})
		}
		if _, ok := err.(gtm.CommonError); !ok {
			logger.Errorf("Error Domain Delete: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Delete error",
				Detail:   err.Error(),
			})
		}
		origErr, ok := err.(gtm.CommonError).GetItem("err").(client.APIError)
		if !ok {
			logger.Errorf("Error Domain Delete: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Delete error",
				Detail:   err.Error(),
			})
		}
		if origErr.Status == 405 && strings.Contains(origErr.RawBody, "Bad Request") && strings.Contains(origErr.RawBody, "DELETE method is not supported") {
			logger.Warnf(": Domain %s delete failed.  Ignoring error (Hashicorp).", d.Id())
		} else {
			logger.Errorf("Error Domain Delete: %s", err.Error())
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Domain Delete error",
				Detail:   err.Error(),
			})
		}
	} else {
		logger.Debugf("Delete status: %v", uStat)
		if uStat.PropagationStatus == "DENIED" {
			logger.Errorf(uStat.Message)
			return append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  uStat.Message,
			})
		}

		waitOnComplete, err := tools.GetBoolValue("wait_on_complete", d)
		if err != nil {
			return diag.FromErr(err)
		}

		if waitOnComplete {
			done, err := waitForCompletion(d.Id(), m)
			if done {
				logger.Infof("Domain Delete completed")
			} else {
				if err == nil {
					logger.Infof("Domain Delete pending")
				} else {
					logger.Errorf("Domain Delete failed [%s]", err.Error())
					return append(diags, diag.Diagnostic{
						Severity: diag.Error,
						Summary:  "Domain Delete failed",
						Detail:   err.Error(),
					})
				}
			}
		}
	}
	d.SetId("")
	return nil

}

// validateDomainType is a SchemaValidateFunc to validate the Domain type.
func validateDomainType(v interface{}, _ string) (ws []string, es []error) {
	value := strings.ToUpper(v.(string))
	if value != "BASIC" && value != "FULL" && value != "WEIGHTED" && value != "STATIC" && value != "FAILOVER-ONLY" {
		es = append(es, fmt.Errorf("type must be basic, full, weighted, static, or failover-only"))
	}
	return
}

// Create and populate a new domain object from resource data
func populateNewDomainObject(d *schema.ResourceData, m interface{}) (*gtm.Domain, error) {

	name, _ := tools.GetStringValue("name", d)
	domObj := gtm.NewDomain(name, d.Get("type").(string))
	err := populateDomainObject(d, domObj, m)

	return domObj, err

}

// Populate existing domain object from resource data
func populateDomainObject(d *schema.ResourceData, dom *gtm.Domain, m interface{}) error {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTM", "populateDomainObject")

	domainName, err := tools.GetStringValue("name", d)
	if err != nil {
		// Should be caught earlier ...
		logger.Warnf("Domain name not set: %s", err.Error())
	}

	if domainName != dom.Name {
		dom.Name = domainName
		logger.Errorf("Domain [%s] state and GTM names inconsistent!", dom.Name)
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetStringValue("type", d); err == nil {
		if v != dom.Type {
			dom.Type = v
		}
	}
	if v, err := tools.GetFloat32Value("default_unreachable_threshold", d); err == nil {
		dom.DefaultUnreachableThreshold = v
	}
	if v, err := tools.GetInterfaceArrayValue("email_notification_list", d); err == nil {
		ls := make([]string, len(v))
		for i, sl := range v {
			ls[i] = sl.(string)
		}
		dom.EmailNotificationList = ls
	} else if d.HasChange("email_notification_list") {
		dom.EmailNotificationList = make([]string, 0)
	}
	if v, err := tools.GetFloat32Value("min_pingable_region_fraction", d); err == nil {
		dom.MinPingableRegionFraction = v
	}
	if v, err := tools.GetIntValue("default_timeout_penalty", d); err == nil || d.HasChange("default_timeout_penalty") {
		dom.DefaultTimeoutPenalty = v
	}
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		logger.Errorf("populateResourceObject() default_timeout_penalty failed: %v", err.Error())
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetIntValue("servermonitor_liveness_count", d); err == nil {
		dom.ServermonitorLivenessCount = v
	}
	if v, err := tools.GetStringValue("round_robin_prefix", d); err == nil {
		dom.RoundRobinPrefix = v
	}
	if v, err := tools.GetIntValue("servermonitor_load_count", d); err == nil {
		dom.ServermonitorLoadCount = v
	}
	if v, err := tools.GetIntValue("ping_interval", d); err == nil {
		dom.PingInterval = v
	}
	if v, err := tools.GetIntValue("max_ttl", d); err == nil {
		dom.MaxTTL = int64(v)
	}
	if v, err := tools.GetFloat64Value("load_imbalance_percentage", d); err == nil {
		dom.LoadImbalancePercentage = v
	}
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		logger.Errorf("populateResourceObject() load_imbalance_percentage failed: %v", err.Error())
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetFloat64Value("default_health_max", d); err == nil {
		dom.DefaultHealthMax = v
	}
	if v, err := tools.GetIntValue("map_update_interval", d); err == nil {
		dom.MapUpdateInterval = v
	}
	if v, err := tools.GetIntValue("max_properties", d); err == nil {
		dom.MaxProperties = v
	}
	if v, err := tools.GetIntValue("max_resources", d); err == nil {
		dom.MaxResources = v
	}
	if v, err := tools.GetStringValue("default_ssl_client_private_key", d); err == nil || d.HasChange("default_ssl_client_private_key") {
		dom.DefaultSslClientPrivateKey = v
	}
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		logger.Errorf("populateResourceObject() default_ssl_client_private_key failed: %v", err.Error())
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetIntValue("default_error_penalty", d); err == nil || d.HasChange("default_error_penalty") {
		dom.DefaultErrorPenalty = v
	}
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		logger.Errorf("populateResourceObject() default_error_penalty failed: %v", err.Error())
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetFloat64Value("max_test_timeout", d); err == nil {
		dom.MaxTestTimeout = v
	}
	if cnameCoalescingEnabled, err := tools.GetBoolValue("cname_coalescing_enabled", d); err == nil {
		dom.CnameCoalescingEnabled = cnameCoalescingEnabled
	}
	if v, err := tools.GetFloat64Value("default_health_multiplier", d); err == nil {
		dom.DefaultHealthMultiplier = v
	}
	if v, err := tools.GetStringValue("servermonitor_pool", d); err == nil {
		dom.ServermonitorPool = v
	}
	if loadFeedback, err := tools.GetBoolValue("load_feedback", d); err == nil {
		dom.LoadFeedback = loadFeedback
	}
	if v, err := tools.GetIntValue("min_ttl", d); err == nil {
		dom.MinTTL = int64(v)
	}
	if v, err := tools.GetIntValue("default_max_unreachable_penalty", d); err == nil {
		dom.DefaultMaxUnreachablePenalty = v
	}
	if v, err := tools.GetFloat64Value("default_health_threshold", d); err == nil {
		dom.DefaultHealthThreshold = v
	}
	if v, err := tools.GetStringValue("comment", d); err == nil || d.HasChange("comment") {
		dom.ModificationComments = v
	}
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		logger.Errorf("populateResourceObject() comment failed: %v", err.Error())
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetIntValue("min_test_interval", d); err == nil {
		dom.MinTestInterval = v
	}
	if v, err := tools.GetIntValue("ping_packet_size", d); err == nil {
		dom.PingPacketSize = v
	}
	if v, err := tools.GetStringValue("default_ssl_client_certificate", d); err == nil || d.HasChange("default_ssl_client_certificate") {
		dom.DefaultSslClientCertificate = v
	}
	if err != nil && !errors.Is(err, tools.ErrNotFound) {
		logger.Errorf("populateResourceObject() default_ssl_client_certificate failed: %v", err.Error())
		return fmt.Errorf("Domain Object could not be populated: %v", err.Error())
	}

	if v, err := tools.GetBoolValue("end_user_mapping_enabled", d); err == nil {
		dom.EndUserMappingEnabled = v
	}

	return nil

}

// Populate Terraform state from provided Domain object
func populateTerraformState(d *schema.ResourceData, dom *gtm.Domain, m interface{}) {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTM", "populateTerraformState")

	for stateKey, stateValue := range map[string]interface{}{
		"name":                            dom.Name,
		"type":                            dom.Type,
		"default_unreachable_threshold":   dom.DefaultUnreachableThreshold,
		"email_notification_list":         dom.EmailNotificationList,
		"min_pingable_region_fraction":    dom.MinPingableRegionFraction,
		"default_timeout_penalty":         dom.DefaultTimeoutPenalty,
		"servermonitor_liveness_count":    dom.ServermonitorLivenessCount,
		"round_robin_prefix":              dom.RoundRobinPrefix,
		"servermonitor_load_count":        dom.ServermonitorLoadCount,
		"ping_interval":                   dom.PingInterval,
		"max_ttl":                         dom.MaxTTL,
		"load_imbalance_percentage":       dom.LoadImbalancePercentage,
		"default_health_max":              dom.DefaultHealthMax,
		"map_update_interval":             dom.MapUpdateInterval,
		"max_properties":                  dom.MaxProperties,
		"max_resources":                   dom.MaxResources,
		"default_ssl_client_private_key":  dom.DefaultSslClientPrivateKey,
		"default_error_penalty":           dom.DefaultErrorPenalty,
		"max_test_timeout":                dom.MaxTestTimeout,
		"cname_coalescing_enabled":        dom.CnameCoalescingEnabled,
		"default_health_multiplier":       dom.DefaultHealthMultiplier,
		"servermonitor_pool":              dom.ServermonitorPool,
		"load_feedback":                   dom.LoadFeedback,
		"min_ttl":                         dom.MinTTL,
		"default_max_unreachable_penalty": dom.DefaultMaxUnreachablePenalty,
		"default_health_threshold":        dom.DefaultHealthThreshold,
		"comment":                         dom.ModificationComments,
		"min_test_interval":               dom.MinTestInterval,
		"ping_packet_size":                dom.PingPacketSize,
		"default_ssl_client_certificate":  dom.DefaultSslClientCertificate,
		"end_user_mapping_enabled":        dom.EndUserMappingEnabled} {
		// walk through all state elements
		err := d.Set(stateKey, stateValue)
		if err != nil {
			logger.Errorf("populateTerraformState failed: %s", err.Error())
		}
	}
}

// Util function to wait for change deployment. return true if complete. false if not - error or nil (timeout)
func waitForCompletion(domain string, m interface{}) (bool, error) {
	meta := akamai.Meta(m)
	logger := meta.Log("Akamai GTMv1", "waitForCompletion")

	var defaultInterval = 5 * time.Second
	var defaultTimeout = 300 * time.Second
	var sleepInterval = defaultInterval // seconds. TODO:Should be configurable by user ...
	var sleepTimeout = defaultTimeout   // seconds. TODO: Should be configurable by user ...
	if HashiAcc {
		// Override for ACC tests
		sleepTimeout = sleepInterval
	}
	logger.Debugf("WAIT: Sleep Interval [%v]", sleepInterval/time.Second)
	logger.Debugf("WAIT: Sleep Timeout [%v]", sleepTimeout/time.Second)
	for {
		propStat, err := gtm.GetDomainStatus(domain)
		if err != nil {
			return false, err
		}
		logger.Debugf("WAIT: propStat.PropagationStatus [%v]", propStat.PropagationStatus)
		switch propStat.PropagationStatus {
		case "COMPLETE":
			logger.Debugf("WAIT: Return COMPLETE")
			return true, nil
		case "DENIED":
			logger.Debugf("WAIT: Return DENIED")
			return false, fmt.Errorf(propStat.Message)
		case "PENDING":
			if sleepTimeout <= 0 {
				logger.Debugf("WAIT: Return TIMED OUT")
				return false, nil
			}
			time.Sleep(sleepInterval)
			sleepTimeout -= sleepInterval
			logger.Debugf("WAIT: Sleep Time Remaining [%v]", sleepTimeout/time.Second)
		default:
			return false, fmt.Errorf("unknown propagationStatus while waiting for change completion") // don't know how/why we would have broken out.
		}
	}
}
