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

func TestAccLocalCommandResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLocalCommandResourceConfig("test_success", "true", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_local_command.test_success", "passed", "true"),
				),
			},
			{
				Config: testAccLocalCommandResourceConfig("test_failure", "false", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_local_command.test_failure", "passed", "false"),
				),
			},
		},
	})
}

func testAccLocalCommandResourceConfig(name string, command string, ignore_failure bool) string {
	return fmt.Sprintf(`
resource "checkmate_local_command" %[1]q {
	command = %[2]q
	create_anyway_on_check_failure = %[3]t
}`, name, command, ignore_failure)

}
