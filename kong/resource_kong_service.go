package kong

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/kevholditch/gokong"
)

func resourceKongService() *schema.Resource {
	return &schema.Resource{
		Create: resourceKongServiceCreate,
		Read:   resourceKongServiceRead,
		Delete: resourceKongServiceDelete,
		Update: resourceKongServiceUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: false,
			},
			"protocol": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: false,
			},
			"host": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			"port": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: false,
				Default:  80,
			},
			"path": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			"retries": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: false,
				Default:  5,
			},
			"connect_timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: false,
				Default:  60000,
			},
			"write_timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: false,
				Default:  60000,
			},
			"read_timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: false,
				Default:  60000,
			},
		},
	}
}

func resourceKongServiceCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*config)
	serviceClient := config.adminClient.Services()

	serviceRequest := createKongServiceRequestFromResourceData(d)

	var serviceID string
	if err := resource.Retry(config.retryTimeout, func() *resource.RetryError {
		log.Printf("creating service %s", *serviceRequest.Name)

		service, err := serviceClient.Create(serviceRequest)
		if err != nil {
			if config.upsertResources && strings.Contains(err.Error(), "unique constraint violation") {
				dbService, err := serviceClient.GetServiceByName(*serviceRequest.Name)
				if err != nil {
					return &resource.RetryError{
						Err:       fmt.Errorf("could not read existing Kong service %s: %w", *serviceRequest.Name, err),
						Retryable: config.retryOnError,
					}
				}
				log.Printf("service named %s already exists with ID: %s, using it", *serviceRequest.Name, *dbService.Id)

				serviceID = *dbService.Id
				return nil
			}
			return &resource.RetryError{
				Err:       fmt.Errorf("failed to create kong service: %v error: %v", serviceRequest, err),
				Retryable: config.retryOnError,
			}
		}
		serviceID = *service.Id
		return nil
	}); err != nil {
		return err
	}

	d.SetId(serviceID)

	return resourceKongServiceRead(d, meta)
}

func resourceKongServiceUpdate(d *schema.ResourceData, meta interface{}) error {
	d.Partial(false)

	serviceRequest := createKongServiceRequestFromResourceData(d)

	_, err := meta.(*config).adminClient.Services().UpdateServiceById(d.Id(), serviceRequest)

	if err != nil {
		return fmt.Errorf("error updating kong service: %s", err)
	}

	return resourceKongServiceRead(d, meta)
}

func resourceKongServiceRead(d *schema.ResourceData, meta interface{}) error {

	service, err := meta.(*config).adminClient.Services().GetServiceById(d.Id())

	if err != nil {
		return fmt.Errorf("could not find kong service: %v", err)
	}

	if service == nil {
		d.SetId("")
	} else {
		if service.Name != nil {
			d.Set("name", service.Name)
		}

		if service.Protocol != nil {
			d.Set("protocol", service.Protocol)
		}

		if service.Host != nil {
			d.Set("host", service.Host)
		}

		if service.Port != nil {
			d.Set("port", service.Port)
		}

		if service.Path != nil {
			d.Set("path", service.Path)
		}

		if service.Retries != nil {
			d.Set("retries", service.Retries)
		}

		if service.ConnectTimeout != nil {
			d.Set("connect_timeout", service.ConnectTimeout)
		}

		if service.WriteTimeout != nil {
			d.Set("write_timeout", service.WriteTimeout)
		}

		if service.ReadTimeout != nil {
			d.Set("read_timeout", service.ReadTimeout)
		}
	}

	return nil
}

func resourceKongServiceDelete(d *schema.ResourceData, meta interface{}) error {

	err := meta.(*config).adminClient.Services().DeleteServiceById(d.Id())

	if err != nil {
		return fmt.Errorf("could not delete kong service: %v", err)
	}

	return nil
}

func createKongServiceRequestFromResourceData(d *schema.ResourceData) *gokong.ServiceRequest {
	return &gokong.ServiceRequest{
		Name:           readStringPtrFromResource(d, "name"),
		Protocol:       readStringPtrFromResource(d, "protocol"),
		Host:           readStringPtrFromResource(d, "host"),
		Port:           readIntPtrFromResource(d, "port"),
		Path:           readStringPtrFromResource(d, "path"),
		Retries:        readIntPtrFromResource(d, "retries"),
		ConnectTimeout: readIntPtrFromResource(d, "connect_timeout"),
		WriteTimeout:   readIntPtrFromResource(d, "write_timeout"),
		ReadTimeout:    readIntPtrFromResource(d, "read_timeout"),
	}
}
