package kong

import (
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/kevholditch/gokong"
)

type config struct {
	adminClient           *gokong.KongAdminClient
	strictPlugins         bool
	strictConsumerPlugins bool
	upsertResources       bool
	retryOnError          bool
	retryTimeout          time.Duration
}

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"kong_admin_uri": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: envDefaultFuncWithDefault("KONG_ADMIN_ADDR", "http://localhost:8001"),
				Description: "The address of the kong admin url e.g. http://localhost:8001",
			},
			"kong_admin_username": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: envDefaultFuncWithDefault("KONG_ADMIN_USERNAME", ""),
				Description: "An basic auth user for kong admin",
			},
			"kong_admin_password": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: envDefaultFuncWithDefault("KONG_ADMIN_PASSWORD", ""),
				Description: "An basic auth password for kong admin",
			},
			"tls_skip_verify": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: envDefaultFuncWithDefault("TLS_SKIP_VERIFY", "false"),
				Description: "Whether to skip tls verify for https kong api endpoint using self signed or untrusted certs",
			},
			"kong_api_key": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: envDefaultFuncWithDefault("KONG_API_KEY", ""),
				Description: "API key for the kong api (if you have locked it down)",
			},
			"kong_admin_token": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: envDefaultFuncWithDefault("KONG_ADMIN_TOKEN", ""),
				Description: "API key for the kong api (Enterprise Edition)",
			},
			"strict_plugins_match": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    false,
				DefaultFunc: envDefaultFuncWithDefault("STRICT_PLUGINS_MATCH", "false"),
				Description: "Should plugins `config_json` field strictly match plugin configuration",
			},
			"upsert_resources": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: envDefaultFuncWithDefault("KONG_UPSERT_RESOURCES", "false"),
				Description: "Use existing resources if creation raises a unique constraint error",
			},
			"retry_on_error": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "If provider encounters an error on resources creation, retry until retry_timeout",
			},
			"retry_timeout": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     20,
				Description: "Timeout in seconds when retrying creation",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"kong_certificate":            resourceKongCertificate(),
			"kong_consumer":               resourceKongConsumer(),
			"kong_consumer_plugin_config": resourceKongConsumerPluginConfig(),
			"kong_plugin":                 resourceKongPlugin(),
			"kong_sni":                    resourceKongSni(),
			"kong_upstream":               resourceKongUpstream(),
			"kong_target":                 resourceKongTarget(),
			"kong_service":                resourceKongService(),
			"kong_route":                  resourceKongRoute(),
		},

		//DataSourcesMap: map[string]*schema.Resource{
		//	"kong_api":         dataSourceKongApi(),
		//	"kong_certificate": dataSourceKongCertificate(),
		//	"kong_consumer":    dataSourceKongConsumer(),
		//	"kong_plugin":      dataSourceKongPlugin(),
		//	"kong_upstream":    dataSourceKongUpstream(),
		//},
		ConfigureFunc: providerConfigure,
	}
}

func envDefaultFuncWithDefault(key string, defaultValue string) schema.SchemaDefaultFunc {
	return func() (interface{}, error) {
		if v := os.Getenv(key); v != "" {
			if v == "true" {
				return true, nil
			} else if v == "false" {
				return false, nil
			}
			return v, nil
		}
		return defaultValue, nil
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	kongConfig := &gokong.Config{
		HostAddress:        d.Get("kong_admin_uri").(string),
		Username:           d.Get("kong_admin_username").(string),
		Password:           d.Get("kong_admin_password").(string),
		InsecureSkipVerify: d.Get("tls_skip_verify").(bool),
		ApiKey:             d.Get("kong_api_key").(string),
		AdminToken:         d.Get("kong_admin_token").(string),
	}

	config := &config{
		adminClient:     gokong.NewClient(kongConfig),
		strictPlugins:   d.Get("strict_plugins_match").(bool),
		upsertResources: d.Get("upsert_resources").(bool),
		retryOnError:    d.Get("retry_on_error").(bool),
		retryTimeout:    time.Duration(d.Get("retry_timeout").(int)) * time.Second,
	}

	return config, nil
}
