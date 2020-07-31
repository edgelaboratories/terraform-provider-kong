package kong

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/kevholditch/gokong"
)

func resourceKongRoute() *schema.Resource {
	return &schema.Resource{
		Create: resourceKongRouteCreate,
		Read:   resourceKongRouteRead,
		Delete: resourceKongRouteDelete,
		Update: resourceKongRouteUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			"protocols": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"methods": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"hosts": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"paths": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"strip_path": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
				Default:  true,
			},
			"source": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: false,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Optional: true,
						},
					},
				},
			},
			"destination": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: false,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Optional: true,
						},
					},
				},
			},
			"snis": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"preserve_host": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: false,
			},
			"regex_priority": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: false,
			},
			"service_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: false,
			},
		},
	}
}

func resourceKongRouteCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*config)
	routeClient := config.adminClient.Routes()

	routeRequest := createKongRouteRequestFromResourceData(d)

	var routeID string
	if err := resource.Retry(config.retryTimeout, func() *resource.RetryError {
		log.Printf("creating route %v", routeRequest)

		route, err := routeClient.Create(routeRequest)
		if err != nil {
			if config.upsertResources && strings.Contains(err.Error(), "unique constraint violation") {
				dbRoute, err := routeClient.GetByName(*routeRequest.Name)
				if err != nil {
					return &resource.RetryError{
						Err:       fmt.Errorf("could not read existing Kong service %s: %w", *routeRequest.Name, err),
						Retryable: config.retryOnError,
					}
				}
				log.Printf("route named %s already exists with ID: %s, using it", *routeRequest.Name, *dbRoute.Id)

				routeID = *route.Id
				return nil
			}

			return &resource.RetryError{
				Err:       fmt.Errorf("failed to create kong route: %v error: %v", routeRequest, err),
				Retryable: config.retryOnError,
			}
		}
		routeID = *route.Id
		return nil
	}); err != nil {
		return err
	}

	d.SetId(routeID)

	return resourceKongRouteRead(d, meta)
}

func resourceKongRouteUpdate(d *schema.ResourceData, meta interface{}) error {
	d.Partial(false)

	routeRequest := createKongRouteRequestFromResourceData(d)

	_, err := meta.(*config).adminClient.Routes().UpdateById(d.Id(), routeRequest)

	if err != nil {
		return fmt.Errorf("error updating kong route: %s", err)
	}

	return resourceKongRouteRead(d, meta)
}

func resourceKongRouteRead(d *schema.ResourceData, meta interface{}) error {

	route, err := meta.(*config).adminClient.Routes().GetById(d.Id())

	if err != nil {
		return fmt.Errorf("could not find kong route: %v", err)
	}

	if route == nil {
		d.SetId("")
	} else {
		if route.Name != nil {
			d.Set("name", route.Name)
		}
		if route.Protocols != nil {
			d.Set("protocols", gokong.StringValueSlice(route.Protocols))
		}

		if route.Methods != nil {
			d.Set("methods", gokong.StringValueSlice(route.Methods))
		}

		if route.Hosts != nil {
			d.Set("hosts", gokong.StringValueSlice(route.Hosts))
		}

		if route.Paths != nil {
			d.Set("paths", gokong.StringValueSlice(route.Paths))
		}

		if route.StripPath != nil {
			d.Set("strip_path", route.StripPath)
		}

		if route.Sources != nil {
			d.Set("source", route.Sources)
		}

		if route.Destinations != nil {
			d.Set("destination", route.Sources)
		}

		if route.PreserveHost != nil {
			d.Set("preserve_host", route.PreserveHost)
		}

		if route.RegexPriority != nil {
			d.Set("regex_priority", route.RegexPriority)
		}

		if route.Snis != nil {
			d.Set("snis", gokong.StringValueSlice(route.Snis))
		}

		if route.Service != nil {
			d.Set("service_id", route.Service)
		}

	}

	return nil
}

func resourceKongRouteDelete(d *schema.ResourceData, meta interface{}) error {

	err := meta.(*config).adminClient.Routes().DeleteById(d.Id())

	if err != nil {
		return fmt.Errorf("could not delete kong route: %v", err)
	}

	return nil
}

func createKongRouteRequestFromResourceData(d *schema.ResourceData) *gokong.RouteRequest {
	return &gokong.RouteRequest{
		Name:          readStringPtrFromResource(d, "name"),
		Protocols:     readStringArrayPtrFromResource(d, "protocols"),
		Methods:       readStringArrayPtrFromResource(d, "methods"),
		Hosts:         readStringArrayPtrFromResource(d, "hosts"),
		Paths:         readStringArrayPtrFromResource(d, "paths"),
		StripPath:     readBoolPtrFromResource(d, "strip_path"),
		Sources:       readIpPortArrayFromResource(d, "source"),
		Destinations:  readIpPortArrayFromResource(d, "destination"),
		PreserveHost:  readBoolPtrFromResource(d, "preserve_host"),
		RegexPriority: readIntPtrFromResource(d, "regex_priority"),
		Snis:          readStringArrayPtrFromResource(d, "snis"),
		Service:       readIdPtrFromResource(d, "service_id"),
	}
}
