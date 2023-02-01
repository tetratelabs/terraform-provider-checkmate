resource "checkmate_http_health" "example" {
  # This is the url of the endpoint we want to check
  url = "http://example.com"

  # We're willing to try up to 10 times
  retries = 10

  # Will perform an HTTP GET request
  method = "GET"

  # The overall test should not take longer than 10 seconds
  timeout = 10000

  # Wait 0.1 seconds between attempts
  interval = 100

  # Expect a status 200 OK
  status_code = 200

  # We want 2 successes in a row
  consecutive_successes = 2

  # Send these HTTP headers
  headers = {
    "Example-Header" = "example value"
  }
}
