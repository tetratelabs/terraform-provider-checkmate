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

default: build

# Run acceptance tests
.PHONY: test
test:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

licenser:
	licenser apply Tetrate -r

build:
	go build -v ./...

install:
	go install

format:
	go fmt ./...

docs: install
	go generate ./...

check: docs licenser format
	[ -z "`git status -uno --porcelain`" ] || (git status && echo 'Check failed. This could be a failed check or dirty git state.'; exit 1)