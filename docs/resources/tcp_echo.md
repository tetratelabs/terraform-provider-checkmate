---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "checkmate_tcp_echo Resource - terraform-provider-checkmate"
subcategory: ""
description: |-
  TCP Echo
---

# checkmate_tcp_echo (Resource)

TCP Echo

## Example Usage

```terraform
resource "checkmate_tcp_echo" "example" {
  # The hostname where the echo request will be sent
  host = "foo.bar"

  # The TCP port at which the request will be sent
  port = 3002

  # Message that will be sent to the TCP echo server
  message = "PROXY tcpbin.com:4242 foobartest"

  # Message expected to be present in the echo response
  expected_message = "foobartest"

  # Set the connection timeout for the destination host, in milliseconds
  connection_timeout = 3000

  # Set the per try timeout for the destination host, in milliseconds
  single_attempt_timeout = 2000

  # Set a number of consecutive sucesses to make the check pass
  consecutive_successes = 5
}

# In case you expect to be some kind of problem, and not getting
# a response back, you can set `expect_failure` to true. In that case
# you can skip `expected_message`.
resource "checkmate_tcp_echo" "example" {
  # The hostname where the echo request will be sent
  host = "foo.bar"

  # The TCP port at which the request will be sent
  port = 3002

  # Message that will be sent to the TCP echo server
  message = "PROXY nonexistent.local:4242 foobartest"

  # Expect this to fail
  expect_write_failure = true

  # Set the connection timeout for the destination host, in milliseconds
  connection_timeout = 3000

  # Set the per try timeout for the destination host, in milliseconds
  single_attempt_timeout = 2000

  # Set a number of consecutive sucesses to make the check pass
  consecutive_successes = 5
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `host` (String) The hostname where to send the TCP echo request to
- `message` (String) The message to send in the echo request
- `port` (Number) The port of the hostname where to send the TCP echo request

### Optional

- `connection_timeout` (Number) The timeout for stablishing a new TCP connection in milliseconds
- `consecutive_successes` (Number) Number of consecutive successes required before the check is considered successful overall. Defaults to 1.
- `create_anyway_on_check_failure` (Boolean) If false, the resource will fail to create if the check does not pass. If true, the resource will be created anyway. Defaults to false.
- `expect_write_failure` (Boolean) Wether or not the check is expected to fail after successfully connecting to the target. If true, the check will be considered successful if it fails. Defaults to false.
- `expected_message` (String) The message expected to be included in the echo response
- `interval` (Number) Interval in milliseconds between attemps. Default 200
- `keepers` (Map of String) Arbitrary map of string values that when changed will cause the check to run again.
- `persistent_response_regex` (String) A regex pattern that the response need to match in every attempt to be considered successful.
  If not provided, the response is not checked.

  If using multiple attempts, this regex will be evaulated against the response text. For every susequent attempt, the regex
  will be evaluated against the response text and compared against the first obtained value. The check will be deemed successful
  if the regex matches the response text in every attempt. A single response not matching such value will cause the check to fail.
- `single_attempt_timeout` (Number) Timeout for an individual attempt. If exceeded, the attempt will be considered failure and potentially retried. Default 5000ms
- `timeout` (Number) Overall timeout in milliseconds for the check before giving up, default 10000

### Read-Only

- `id` (String) Identifier
- `passed` (Boolean) True if the check passed
