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

import "time"

type RetryWindow struct {
	MaxRetries           int
	Timeout              time.Duration
	Interval             time.Duration
	ConsecutiveSuccesses int
}

type RetryResult int

const (
	Success RetryResult = iota
	RetriesExceeded
	TimeoutExceeded
)

func (r *RetryWindow) Do(action func(attempt int, succeses int) bool) RetryResult {
	success := make(chan bool)
	lastTry := make(chan bool)
	go func() {
		successCount := 0
		retries := 0
		// The loop is infinite if MaxRetries is 0
		for r.MaxRetries == 0 || retries < r.MaxRetries {
			if action(retries, successCount) {
				successCount++
				retries = 0 // reset retries
				if successCount >= r.ConsecutiveSuccesses {
					success <- true
					return
				}
			} else {
				successCount = 0
				retries++
			}
			time.Sleep(r.Interval)
		}

	}()

	select {
	case <-success:
		return Success
	case <-time.After(r.Timeout):
		return TimeoutExceeded
	case <-lastTry:
		return RetriesExceeded
	}
}
