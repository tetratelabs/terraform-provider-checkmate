// Copyright 2024 Tetrate
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

package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	// TODO: add some cases that validate the mutated data and diagnostic parameters.
	tests := []struct {
		name    string
		args    *HttpHealthArgs
		mock    func(w http.ResponseWriter, r *http.Request)
		wantErr bool
	}{
		{
			name: "errors on timeout",
			args: &HttpHealthArgs{
				Method:               "GET",
				Timeout:              1000,
				ConsecutiveSuccesses: 2,
				StatusCode:           "200",
				JSONPath:             "{.SomeField}",
				JSONValue:            "someValue",
			},
			mock: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"SomeField": "notSomeValue"}`))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mock))
			defer server.Close()
			tt.args.URL = server.URL

			err := HealthCheck(context.Background(), tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("HealthCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

}
