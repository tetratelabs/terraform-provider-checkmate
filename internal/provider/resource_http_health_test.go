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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccHttpHealthResource(t *testing.T) {
	testUrl := "http://example.com"
	// testUrl2 := "https://httpbin.org/get"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccHttpHealthResourceConfig(testUrl),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_http_health.test", "url", testUrl),
					resource.TestCheckResourceAttr("checkmate_http_health.test", "id", "example-id"),
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

func testAccHttpHealthResourceConfig(url string) string {
	return fmt.Sprintf(`
resource "checkmate_http_health" "test" {
  url = %[1]q
  consecutive_successes = 5
  headers = {
	hello = "world"
  }
}
`, url)
}
