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

func TestAccTCPEchoResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTCPEchoResourceConfig("test_success", "tcpbin.com", 4242, "foobar", "foobar", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_tcp_echo.test_success", "passed", "true"),
				),
			},
			{
				Config: testAccTCPEchoResourceConfig("test_failure", "foo.bar", 1234, "foobar", "foobar", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_tcp_echo.test_failure", "passed", "false"),
				),
			},
			{
				Config: testAccTCPEchoResourceConfig("test_failure", "foo.bar", 1234, "foobar", "foobar", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_tcp_echo.test_failure", "passed", "false"),
				),
			},
			{
				Config: testTCPEchoResourceRegex("test_regex_ok", `\(.*\)`, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_tcp_echo.test_regex_ok", "passed", "true"),
				),
			},
			{
				Config: testTCPEchoResourceRegex("test_regex_not_match_pass_anyway", "test", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_tcp_echo.test_regex_not_match_pass_anyway", "passed", "false"),
				),
			},
		},
	})
}

func testAccTCPEchoResourceConfig(name, host string, port int, message, expected_message string, ignore_failure bool) string {
	return fmt.Sprintf(`
resource "checkmate_tcp_echo" %q {
	host = %q
	port = %d
	message = %q
	timeout = 1000
	expected_message = %q
	create_anyway_on_check_failure = %t
}`, name, host, port, message, expected_message, ignore_failure)

}

func testTCPEchoResourceRegex(name, regex string, ignore_failure bool) string {
	return fmt.Sprintf(`
resource "checkmate_tcp_echo" %q {
	host = "tcpbin.platform.tetrate.com"
	port = 15080
	message = "foobar (123)"
	timeout = 1000 * 10
	expected_message = "foobar (123)"
	persistent_response_regex = %q
	create_anyway_on_check_failure = %t
	consecutive_successes = 2
}`, name, regex, ignore_failure)

}
