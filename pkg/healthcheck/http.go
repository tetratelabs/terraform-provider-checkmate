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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tetratelabs/terraform-provider-checkmate/pkg/helpers"
	"k8s.io/client-go/util/jsonpath"
)

type HttpHealthArgs struct {
	URL                  string
	Method               string
	Timeout              int64
	RequestTimeout       int64
	Interval             int64
	StatusCode           string
	ConsecutiveSuccesses int64
	Headers              map[string]string
	IgnoreFailure        bool
	Passed               bool
	RequestBody          string
	ResultBody           string
	CABundle             string
	InsecureTLS          bool
	JSONPath             string
	JSONValue            string
}

func HealthCheck(ctx context.Context, data *HttpHealthArgs, diag *diag.Diagnostics) error {
	var err error

	data.Passed = false
	endpoint, err := url.Parse(data.URL)
	if err != nil {
		diagAddError(diag, "Client Error", fmt.Sprintf("Unable to parse url %q, got error %s", data.URL, err))
		return fmt.Errorf("parse url %q: %w", data.URL, err)
	}

	if (data.JSONPath != "" && data.JSONValue == "") || (data.JSONPath == "" && data.JSONValue != "") {
		diagAddError(diag, "Client Error", "Both JSONPath and JSONValue must be specified")
		return errors.New("both JSONPath and JSONValue must be specified")
	}

	var checkCode func(int) (bool, error)
	// check the pattern once
	checkStatusCode(data.StatusCode, 0, diag)
	if diag.HasError() {
		return fmt.Errorf("bad status code pattern")
	}
	checkCode = func(c int) (bool, error) { return checkStatusCode(data.StatusCode, c, diag) }

	// normalize headers
	headers := make(map[string][]string)
	if data.Headers != nil {
		for k, v := range data.Headers {
			headers[k] = []string{v}
		}
	}

	window := helpers.RetryWindow{
		Timeout:              time.Duration(data.Timeout) * time.Millisecond,
		Interval:             time.Duration(data.Interval) * time.Millisecond,
		ConsecutiveSuccesses: int(data.ConsecutiveSuccesses),
	}
	data.ResultBody = ""

	if data.CABundle != "" && data.InsecureTLS {
		diagAddError(diag, "Conflicting configuration", "You cannot specify both custom CA and insecure TLS. Please use only one of them.")
	}
	tlsConfig := &tls.Config{}
	if data.CABundle != "" {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM([]byte(data.CABundle)); !ok {
			diagAddError(diag, "Building CA cert pool", err.Error())
			multierror.Append(err, fmt.Errorf("build CA cert pool: %w", err))
		}
		tlsConfig.RootCAs = caCertPool
	}
	tlsConfig.InsecureSkipVerify = data.InsecureTLS

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: time.Duration(data.RequestTimeout) * time.Millisecond,
	}

	tflog.Debug(ctx, fmt.Sprintf("Starting HTTP health check. Overall timeout: %d ms, request timeout: %d ms", data.Timeout, data.RequestTimeout))
	for h, v := range headers {
		tflog.Debug(ctx, fmt.Sprintf("%s: %s", h, v))
	}

	result := window.Do(func(attempt int, successes int) bool {
		if successes != 0 {
			tflog.Trace(ctx, fmt.Sprintf("SUCCESS [%d/%d] http %s %s", successes, data.ConsecutiveSuccesses, data.Method, endpoint))
		} else {
			tflog.Trace(ctx, fmt.Sprintf("ATTEMPT #%d http %s %s", attempt, data.Method, endpoint))
		}

		httpResponse, err := client.Do(&http.Request{
			URL:    endpoint,
			Method: data.Method,
			Header: headers,
			Body:   io.NopCloser(strings.NewReader(data.RequestBody)),
		})
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("CONNECTION FAILURE %v", err))
			return false
		}

		success, err := checkCode(httpResponse.StatusCode)
		if err != nil {
			diagAddError(diag, "check status code", err.Error())
			multierror.Append(err, fmt.Errorf("check status code: %w", err))
		}
		if success {
			tflog.Trace(ctx, fmt.Sprintf("SUCCESS CODE %d", httpResponse.StatusCode))
			body, err := io.ReadAll(httpResponse.Body)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("ERROR READING BODY %v", err))
				data.ResultBody = ""
			} else {
				tflog.Warn(ctx, fmt.Sprintf("READ %d BYTES", len(body)))
				data.ResultBody = string(body)
			}
		} else {
			tflog.Trace(ctx, fmt.Sprintf("FAILURE CODE %d", httpResponse.StatusCode))
		}

		// Check JSONPath
		j := jsonpath.New("parser")
		err = j.Parse(data.JSONPath)
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("ERROR PARSING JSONPATH EXPRESSION %v", err))
			return false
		}
		var respJSON interface{}
		json.Unmarshal([]byte(data.ResultBody), &respJSON)
		buf := new(bytes.Buffer)
		err = j.Execute(buf, respJSON)
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("ERROR EXECUTING JSONPATH %v", err))
			return false
		}

		re := regexp.MustCompile(data.JSONValue)
		return re.MatchString(buf.String())
	})

	switch result {
	case helpers.Success:
		data.Passed = true
	case helpers.TimeoutExceeded:
		diagAddWarning(diag, "Timeout exceeded", fmt.Sprintf("Timeout of %d milliseconds exceeded", data.Timeout))
		if !data.IgnoreFailure {
			diagAddError(diag, "Check failed", "The check did not pass within the timeout and create_anyway_on_check_failure is false")
			multierror.Append(err, fmt.Errorf("the check did not pass within the timeout and create_anyway_on_check_failure is false"))
		}
	}

	return err
}

func checkStatusCode(pattern string, code int, diag *diag.Diagnostics) (bool, error) {
	ranges := strings.Split(pattern, ",")
	for _, r := range ranges {
		bounds := strings.Split(r, "-")
		if len(bounds) == 2 {
			left, err := strconv.Atoi(bounds[0])
			if err != nil {
				diagAddError(diag, "Bad status code pattern", fmt.Sprintf("Can't convert %s to integer. %s", bounds[0], err))
				return false, fmt.Errorf("convert %q to integer: %w", bounds[0], err)
			}
			right, err := strconv.Atoi(bounds[1])
			if err != nil {
				diagAddError(diag, "Bad status code pattern", fmt.Sprintf("Can't convert %s to integer. %s", bounds[1], err))
				return false, fmt.Errorf("convert %q to integer: %w", bounds[0], err)
			}
			if left > right {
				diagAddError(diag, "Bad status code pattern", fmt.Sprintf("Left bound %d is greater than right bound %d", left, right))
				return false, fmt.Errorf("left bound %d is greater than right bound %d", left, right)
			}
			if left <= code && right >= code {
				return true, nil
			}
		} else if len(bounds) == 1 {
			val, err := strconv.Atoi(bounds[0])
			if err != nil {
				diagAddError(diag, "Bad status code pattern", fmt.Sprintf("Can't convert %s to integer. %s", bounds[0], err))
				return false, fmt.Errorf("convert %q to integer: %w", bounds[0], err)
			}
			if val == code {
				return true, nil
			}
		} else {
			diagAddError(diag, "Bad status code pattern", "Too many dashes in range pattern")
			return false, errors.New("too many dashes in range pattern")
		}
	}
	return false, errors.New("status code does not match pattern")
}

func diagAddError(diag *diag.Diagnostics, summary string, details string) {
	if diag != nil {
		diag.AddError(summary, details)
	}
}

func diagAddWarning(diag *diag.Diagnostics, summary string, details string) {
	if diag != nil {
		diag.AddWarning(summary, details)
	}
}
