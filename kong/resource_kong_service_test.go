package kong

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/kevholditch/gokong"
)

func TestAccKongService(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKongServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testCreateServiceConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKongServiceExists("kong_service.service"),
					resource.TestCheckResourceAttr("kong_service.service", "name", "test"),
					resource.TestCheckResourceAttr("kong_service.service", "protocol", "http"),
					resource.TestCheckResourceAttr("kong_service.service", "host", "test.org"),
					resource.TestCheckResourceAttr("kong_service.service", "port", "80"),
					resource.TestCheckResourceAttr("kong_service.service", "path", "/mypath"),
					resource.TestCheckResourceAttr("kong_service.service", "retries", "5"),
					resource.TestCheckResourceAttr("kong_service.service", "connect_timeout", "1000"),
					resource.TestCheckResourceAttr("kong_service.service", "write_timeout", "2000"),
					resource.TestCheckResourceAttr("kong_service.service", "read_timeout", "3000"),
				),
			},
			{
				Config: testUpdateServiceConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKongServiceExists("kong_service.service"),
					resource.TestCheckResourceAttr("kong_service.service", "name", "test2"),
					resource.TestCheckResourceAttr("kong_service.service", "protocol", "https"),
					resource.TestCheckResourceAttr("kong_service.service", "host", "test2.org"),
					resource.TestCheckResourceAttr("kong_service.service", "path", "/"),
					resource.TestCheckResourceAttr("kong_service.service", "retries", "5"),
					resource.TestCheckResourceAttr("kong_service.service", "connect_timeout", "6000"),
					resource.TestCheckResourceAttr("kong_service.service", "write_timeout", "5000"),
					resource.TestCheckResourceAttr("kong_service.service", "read_timeout", "4000"),
				),
			},
		},
	})

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKongServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testCreateServiceConfigZero,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKongServiceExists("kong_service.service"),
					resource.TestCheckResourceAttr("kong_service.service", "retries", "0"),
				),
			},
			{
				Config: testUpdateServiceConfigZero,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKongServiceExists("kong_service.service"),
					resource.TestCheckResourceAttr("kong_service.service", "retries", "0"),
				),
			},
		},
	})
}

func TestAccKongServiceImport(t *testing.T) {

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKongServiceDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testImportServiceConfig,
			},

			resource.TestStep{
				ResourceName:      "kong_service.service",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccKongService_Upsert(t *testing.T) {

	uniqueConstraintError, _ := regexp.Compile(".*unique constraint violation.*")

	TestProvider_configure(t)

	os.Unsetenv("KONG_UPSERT_RESOURCES")
	defer os.Unsetenv("KONG_UPSERT_RESOURCES")

	// Simulate that service with same name already exists but with different values.
	request := &gokong.ServiceRequest{
		Name:     gokong.String("test"),
		Host:     gokong.String("foo.org"),
		Protocol: gokong.String("https"),
	}
	service, err := testAccProvider.Meta().(*config).adminClient.Services().Create(request)
	if err != nil {
		t.Fatalf("could not create service test: %v", err)
	}
	expectedServiceID := *service.Id

	serviceConf := `
resource "kong_service" "service" {
	name     = "test"
	host     = "test.org"
	protocol = "http"
}`
	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			// With upsert disable (default value), this will raise a unique constraint error.
			{
				Config:      serviceConf,
				ExpectError: uniqueConstraintError,
			},
			// enable upsert, this should update the existing
			{
				PreConfig: func() {
					err := os.Setenv("KONG_UPSERT_RESOURCES", "true")
					if err != nil {
						t.Fatalf("Could not set KONG_UPSERT_RESOURCES env variable: %v", err)
					}
				},
				Config: serviceConf,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKongServiceExists("kong_service.service"),
					resource.TestCheckResourceAttr("kong_service.service", "id", expectedServiceID),
					resource.TestCheckResourceAttr("kong_service.service", "host", "test.org"),
					resource.TestCheckResourceAttr("kong_service.service", "protocol", "http"),
				),
			},
		},
	})
}

func testAccCheckKongServiceDestroy(state *terraform.State) error {

	client := testAccProvider.Meta().(*config).adminClient

	services := getResourcesByType("kong_service", state)

	if len(services) != 1 {
		return fmt.Errorf("expecting only 1 service resource found %v", len(services))
	}

	response, err := client.Services().GetServiceById(services[0].Primary.ID)

	if err != nil {
		return fmt.Errorf("error calling get service by id: %v", err)
	}

	if response != nil {
		return fmt.Errorf("service %s still exists, %+v", services[0].Primary.ID, response)
	}

	return nil
}

func testAccCheckKongServiceExists(resourceKey string) resource.TestCheckFunc {

	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceKey]

		if !ok {
			return fmt.Errorf("not found: %s", resourceKey)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID is set")
		}

		service, err := testAccProvider.Meta().(*config).adminClient.Services().GetServiceById(rs.Primary.ID)

		if err != nil {
			return err
		}

		if service == nil {
			return fmt.Errorf("service with id %v not found", rs.Primary.ID)
		}

		return nil
	}
}

const testCreateServiceConfig = `
resource "kong_service" "service" {
	name     		= "test"
	protocol 		= "http"
	host     		= "test.org"
	path     		= "/mypath"
	retries  		= 5
	connect_timeout = 1000
	write_timeout 	= 2000
	read_timeout  	= 3000
	
}
`
const testUpdateServiceConfig = `
resource "kong_service" "service" {
	name     		= "test2"
	protocol 		= "https"
	host     		= "test2.org"
	port     		= 8081
	path     		= "/"
	connect_timeout = 6000
	write_timeout 	= 5000
	read_timeout  	= 4000
}
`
const testCreateServiceConfigZero = `
resource "kong_service" "service" {
	name     		= "test"
	protocol 		= "http"
	host     		= "test.org"
	path     		= "/mypath"
	retries  		= 0
	connect_timeout = 1000
	write_timeout 	= 2000
	read_timeout  	= 3000
}
`
const testUpdateServiceConfigZero = `
resource "kong_service" "service" {
	name     		= "test2"
	protocol 		= "https"
	host     		= "test2.org"
	port     		= 8081
	path     		= "/"
	connect_timeout = 6000
	write_timeout 	= 5000
	read_timeout  	= 4000
	retries         = 0
}
`
const testImportServiceConfig = `
resource "kong_service" "service" {
	name     		= "test"
	protocol 		= "http"
	host     		= "test.org"
	port     		= 8080
	path     		= "/mypath"
	retries  		= 5
	connect_timeout = 8000
	write_timeout 	= 9000
	read_timeout  	= 10000
}
`
