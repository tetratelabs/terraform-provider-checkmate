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

package helpers

import (
	"context"
	"time"
)

type RetryWindow struct {
	Context              context.Context
	Timeout              time.Duration
	Interval             time.Duration
	ConsecutiveSuccesses int
}

type RetryResult int

const (
	Success RetryResult = iota
	TimeoutExceeded
	Failure
)

func (r *RetryWindow) Do(action func(attempt int, successes int) bool) RetryResult {
	success := make(chan struct{})
	failure := make(chan struct{})
	go func() {
		attempt := 0
		successCount := 0
		// run a while true loop, exiting when the timeout expires
		for {
			select {
			case <-r.Context.Done():
				failure <- struct{}{}
				return
			default:
				attempt++
				if action(attempt, successCount) {
					successCount++
					if successCount >= r.ConsecutiveSuccesses {
						success <- struct{}{}
						return
					}
				} else {
					successCount = 0
				}
				time.Sleep(r.Interval)
			}
		}
	}()

	select {
	case <-success:
		return Success
	case <-failure:
		return Failure
	case <-time.After(r.Timeout):
		return TimeoutExceeded
	}
}
