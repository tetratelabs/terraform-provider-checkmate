// Copyright 2023 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccHttpHealthResource(t *testing.T) {
	// testUrl := "http://example.com"
	testUrl := "https://httpbin.org/status/200"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccHttpHealthResourceConfig("test", testUrl),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_http_health.test", "url", testUrl),
				),
			},
			{
				Config: testAccHttpHealthResourceConfig("test_headers", "https://httpbin.org/headers"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("checkmate_http_health.test_headers", "result_body", checkHeader("Hello", "world")),
				),
			},
			{
				Config: testAccHttpHealthResourceConfigWithBody("test_post", "https://httpbin.org/post", "hello"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("checkmate_http_health.test_post", "result_body", checkResponse("hello")),
				),
			},
			// ImportState testing
			// {
			// 	ResourceName:      "checkmate_http_health.test",
			// 	ImportState:       true,
			// 	ImportStateVerify: true,
			// },
			// Update and Read testing
			// {
			// 	Config: testAccHttpHealthResourceConfig(testUrl2),
			// 	Check: resource.ComposeAggregateTestCheckFunc(
			// 		resource.TestCheckResourceAttr("checkmate_http_health.test", "url", testUrl2),
			// 	),
			// },
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccHttpHealthResourceConfig(name string, url string) string {
	return fmt.Sprintf(`
resource "checkmate_http_health" %[1]q {
  url = %[2]q
  consecutive_successes = 1
  headers = {
	hello = "world"
  }
}
`, name, url)
}
func testAccHttpHealthResourceConfigWithBody(name string, url string, body string) string {
	return fmt.Sprintf(`
resource "checkmate_http_health" %[1]q {
  url = %[2]q
  consecutive_successes = 1
  method = "POST"
  headers = {
	"Content-Type" = "application/text"
  }
  request_body = %[3]q
}
`, name, url, body)

}

func checkHeader(key string, value string) func(string) error {
	return func(responseBody string) error {
		var parsed map[string]map[string]string
		if err := json.Unmarshal([]byte(responseBody), &parsed); err != nil {
			return err
		}
		if val, ok := parsed["headers"][key]; ok {
			if val == value {
				return nil
			}
			return fmt.Errorf("Key %q exists but value %q does not match", key, val)
		}
		return fmt.Errorf("Key %q does not exist in returned headers", key)
	}
}

func checkResponse(value string) func(string) error {
	return func(responseBody string) error {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(responseBody), &parsed); err != nil {
			return err
		}
		if val, ok := parsed["data"]; ok {
			if val == value {
				return nil
			}
			return fmt.Errorf("Value returned %q does not match %q", parsed["data"], val)
		}
		return fmt.Errorf("Bad response from httpbin")
	}
}
