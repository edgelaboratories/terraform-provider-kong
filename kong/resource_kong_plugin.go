package kong

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/kevholditch/gokong"
)

func resourceKongPlugin() *schema.Resource {
	return &schema.Resource{
		Create: resourceKongPluginCreate,
		Read:   resourceKongPluginRead,
		Delete: resourceKongPluginDelete,
		Update: resourceKongPluginUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"consumer_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			"service_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			"route_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			"enabled": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
				Default:  true,
			},
			"config_json": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				StateFunc:    normalizeDataJSON,
				ValidateFunc: validateDataJSON,
				Description:  "plugin configuration in JSON format, configuration must be a valid JSON object.",
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return new == ""
				},
			},
			"strict_match": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
				Default:  false,
			},
			"computed_config": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceKongPluginCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*config)
	pluginClient := config.adminClient.Plugins()

	pluginRequest, err := createKongPluginRequestFromResourceData(d)
	if err != nil {
		return err
	}

	var pluginID string
	if err = resource.Retry(config.retryTimeout, func() *resource.RetryError {
		log.Printf("creating plugin %s", pluginRequest.Name)

		plugin, err := pluginClient.Create(pluginRequest)
		if err != nil {
			if config.upsertResources && strings.Contains(err.Error(), "unique constraint violation") {
				dbPlugin, err := findPlugin(
					pluginClient, pluginRequest.Name, pluginRequest.ConsumerId, pluginRequest.RouteId, pluginRequest.ServiceId,
				)
				if err != nil {
					return &resource.RetryError{
						Err:       err,
						Retryable: config.retryOnError,
					}
				}

				pluginID = dbPlugin.Id
				return nil
			}
			return &resource.RetryError{
				Err:       fmt.Errorf("failed to create kong plugin: %v error: %w", pluginRequest, err),
				Retryable: config.retryOnError,
			}
		}
		pluginID = plugin.Id
		return nil
	}); err != nil {
		return err
	}

	d.SetId(pluginID)

	return resourceKongPluginRead(d, meta)
}

func resourceKongPluginUpdate(d *schema.ResourceData, meta interface{}) error {
	d.Partial(false)

	pluginRequest, err := createKongPluginRequestFromResourceData(d)
	if err != nil {
		return err
	}

	_, err = meta.(*config).adminClient.Plugins().UpdateById(d.Id(), pluginRequest)

	if err != nil {
		return fmt.Errorf("error updating kong plugin: %s", err)
	}

	return resourceKongPluginRead(d, meta)
}

func resourceKongPluginRead(d *schema.ResourceData, meta interface{}) error {

	plugin, err := meta.(*config).adminClient.Plugins().GetById(d.Id())

	if err != nil {
		return fmt.Errorf("could not find kong plugin: %v", err)
	}

	if plugin == nil {
		d.SetId("")
	} else {
		d.Set("name", plugin.Name)
		d.Set("service_id", plugin.ServiceId)
		d.Set("route_id", plugin.RouteId)
		d.Set("consumer_id", plugin.ConsumerId)
		d.Set("enabled", plugin.Enabled)

		// We sync this property from upstream as a method to allow you to import a resource with the config tracked in
		// terraform state. We do not track `config` as it will be a source of a perpetual diff.
		// https://www.terraform.io/docs/extend/best-practices/detecting-drift.html#capture-all-state-in-read
		upstreamJson := pluginConfigJsonToString(plugin.Config)
		setConfig := func(strict bool) {
			if strict {
				d.Set("config_json", upstreamJson)
			} else {
				d.Set("computed_config", upstreamJson)
			}
		}
		if value, ok := d.GetOk("strict_match"); ok {
			setConfig(value.(bool))
		} else {
			setConfig(meta.(*config).strictPlugins)
		}
	}

	return nil
}

func resourceKongPluginDelete(d *schema.ResourceData, meta interface{}) error {

	err := meta.(*config).adminClient.Plugins().DeleteById(d.Id())

	if err != nil {
		return fmt.Errorf("could not delete kong plugin: %v", err)
	}

	return nil
}

func createKongPluginRequestFromResourceData(d *schema.ResourceData) (*gokong.PluginRequest, error) {

	pluginRequest := &gokong.PluginRequest{}

	pluginRequest.Name = readStringFromResource(d, "name")
	pluginRequest.ConsumerId = readIdPtrFromResource(d, "consumer_id")
	pluginRequest.ServiceId = readIdPtrFromResource(d, "service_id")
	pluginRequest.RouteId = readIdPtrFromResource(d, "route_id")
	pluginRequest.Enabled = readBoolPtrFromResource(d, "enabled")

	if data, ok := d.GetOk("config_json"); ok {
		var configJson map[string]interface{}

		err := json.Unmarshal([]byte(data.(string)), &configJson)
		if err != nil {
			return pluginRequest, fmt.Errorf("failed to unmarshal config_json, err: %v", err)
		}

		pluginRequest.Config = configJson
	}

	return pluginRequest, nil
}

// Since this config is a schemaless "blob" we have to remove computed properties
func pluginConfigJsonToString(data map[string]interface{}) string {
	marshalledData := map[string]interface{}{}
	for key, val := range data {
		if !contains(computedPluginProperties, key) {
			marshalledData[key] = val
		}
	}
	// We know it is valid JSON at this point
	rawJson, _ := json.Marshal(marshalledData)

	return string(rawJson)
}

func findPlugin(
	pluginClient *gokong.PluginClient, name string, consumerId *gokong.Id, routeId *gokong.Id, serviceId *gokong.Id,
) (*gokong.Plugin, error) {
	// Size is just how many plugins per request (1000 is the max)
	// but List will fetch all the pages so all the plugins
	dbPlugins, err := pluginClient.List(&gokong.PluginQueryString{Size: 1000})
	if err != nil {
		return nil, fmt.Errorf("could not read existing plugins: %w", err)
	}
	for _, p := range dbPlugins {
		if p.Name == name &&
			gokong.IdToString(p.ConsumerId) == gokong.IdToString(consumerId) &&
			gokong.IdToString(p.RouteId) == gokong.IdToString(routeId) &&
			gokong.IdToString(p.ServiceId) == gokong.IdToString(serviceId) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("could not find plugin with name %s", name)
}
