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
