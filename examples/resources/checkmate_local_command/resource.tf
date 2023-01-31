# Copyright 2023 Tetrate
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

resource "checkmate_local_command" "example" {
  # Run this command in a shell
  command = "python fancy_script.py"

  # Switch to this directory before running the command
  working_directory = "./scripts"

  # We're willing to try up to 10 times
  retries = 5

  # The overall test should not take longer than 5 seconds
  timeout = 5000

  # Wait 0.1 seconds between attempts
  interval = 100

  # We want 2 successes in a row
  consecutive_successes = 1

}
