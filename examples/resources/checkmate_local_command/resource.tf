resource "checkmate_local_command" "example" {
  # Run this command in a shell
  command = "python3 $CHECKMATE_FILEPATH"

  # Switch to this directory before running the command
  working_directory = "./scripts"

  # The overall test should not take longer than 5 seconds
  timeout = 5000

  # Wait 0.1 seconds between attempts
  interval = 100

  # We want 2 successes in a row
  consecutive_successes = 2

  # Create the script file before running the attempts
  create_file = {
    name             = "fancy_script.py"
    contents         = "print('hello world')"
    use_working_dir  = true
    create_directory = true
  }

  create_anyway_on_check_failure = false
}

output "stdout" {
  value = checkmate_local_command.example.stdout
}

output "stderr" {
  value = checkmate_local_command.example.stderr
}