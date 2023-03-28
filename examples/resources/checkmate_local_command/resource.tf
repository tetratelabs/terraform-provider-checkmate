resource "checkmate_local_command" "example" {
  # Run this command in a shell
  command = "python fancy_script.py"

  # Switch to this directory before running the command
  working_directory = "./scripts"

  # The overall test should not take longer than 5 seconds
  timeout = 5000

  # Wait 0.1 seconds between attempts
  interval = 100

  # We want 2 successes in a row
  consecutive_successes = 1

}
