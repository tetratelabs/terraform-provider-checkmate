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
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccHttpHealthResource(t *testing.T) {
	// testUrl := "http://example.com"
	timeout := 6000 // 6s
	httpBin, envExists := os.LookupEnv("HTTPBIN")
	if !envExists {
		httpBin = "https://httpbin.org"
	}
	url200 := httpBin + "/status/200"
	urlPost := httpBin + "/post"
	urlHeaders := httpBin + "/headers"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccHttpHealthResourceConfig("test", url200, timeout),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("checkmate_http_health.test", "url", url200),
				),
			},
			{
				Config: testAccHttpHealthResourceConfig("test_headers", urlHeaders, timeout),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("checkmate_http_health.test_headers", "result_body", checkHeader("Hello", "world")),
				),
			},
			{
				Config: testAccHttpHealthResourceConfigWithBody("test_post", urlPost, "hello", timeout),
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

func TestStatusCodePattern(t *testing.T) {
	tests := []struct {
		pattern string
		code    int
		want    bool
		wantErr bool
	}{
		{"200", 200, true, false},
		{"200-204,300-305", 204, true, false},
		{"200-204,300-305", 299, false, false},
		{"foo", 200, false, true},
		{"200-204,204-300", 200, true, false},
		{"200-204-300", 200, false, true},
		{"200,,", 0, false, true},
		{"--200", 0, false, true},
	}
	for _, tt := range tests {
		diag := &diag.Diagnostics{}
		got := checkStatusCode(tt.pattern, tt.code, diag)
		if got != tt.want {
			t.Errorf("checkStatusCode(%q, %d) got %v, want %v", tt.pattern, tt.code, got, tt.want)
		}
		if tt.wantErr {
			if !diag.HasError() {
				t.Errorf("checkStatusCode(%q, %d) expected an error, but got none", tt.pattern, tt.code)
			}
		} else {
			if diag.HasError() {
				t.Errorf("checkStatusCode(%q, %d) got unexpected errors: %v", tt.pattern, tt.code, diag)
			}
		}
	}
}

func testAccHttpHealthResourceConfig(name string, url string, timeout int) string {
	return fmt.Sprintf(`
resource "checkmate_http_health" %[1]q {
  url = %[2]q
  consecutive_successes = 1
  headers = {
	hello = "world"
  }
  timeout = %[3]d
}
`, name, url, timeout)
}
func testAccHttpHealthResourceConfigWithBody(name string, url string, body string, timeout int) string {
	return fmt.Sprintf(`
resource "checkmate_http_health" %[1]q {
  url = %[2]q
  consecutive_successes = 1
  method = "POST"
  headers = {
	"Content-Type" = "application/text"
  }
  request_body = %[3]q
  timeout = %[4]d
}
`, name, url, body, timeout)

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
