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

package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/tetratelabs/terraform-provider-checkmate/internal/helpers"
	"github.com/tetratelabs/terraform-provider-checkmate/internal/modifiers"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &HttpHealthResource{}
var _ resource.ResourceWithImportState = &HttpHealthResource{}

func NewHttpHealthResource() resource.Resource {
	return &HttpHealthResource{}
}

type HttpHealthResource struct{}

// Schema implements resource.Resource
func (*HttpHealthResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "HTTPS Healthcheck",

		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "URL",
				Required:            true,
			},
			"method": schema.StringAttribute{
				MarkdownDescription: "HTTP Method, defaults to GET",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{modifiers.DefaultString("GET")},
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "Overall timeout in milliseconds for the check before giving up. Default 5000",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(5000)},
			},
			"request_timeout": schema.Int64Attribute{
				MarkdownDescription: "Timeout for an individual request. If exceeded, the attempt will be considered failure and potentially retried. Default 1000",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(1000)},
			},
			"interval": schema.Int64Attribute{
				MarkdownDescription: "Interval in milliseconds between attemps. Default 200",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(200)},
			},
			"status_code": schema.StringAttribute{
				MarkdownDescription: "Status Code to expect. Default 200",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{modifiers.DefaultString("200")},
			},
			"consecutive_successes": schema.Int64Attribute{
				MarkdownDescription: "Number of consecutive successes required before the check is considered successful overall. Defaults to 1.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(1)},
			},
			"headers": schema.MapAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "HTTP Request Headers",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"request_body": schema.StringAttribute{
				MarkdownDescription: "Optional request body to send on each attempt.",
				Optional:            true,
			},
			"result_body": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Result body",
			},
			"passed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the check passed",
			},
			"create_anyway_on_check_failure": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "If false, the resource will fail to create if the check does not pass. If true, the resource will be created anyway. Defaults to false.",
			},
			"ca_bundle": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The CA bundle to use when connecting to the target host.",
			},
			"insecure_tls": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Wether or not to completely skip the TLS CA verification. Default false.",
			},
			"keepers": schema.MapAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Arbitrary map of string values that when changed will cause the healthcheck to run again.",
				Optional:            true,
			},
		},
	}
}

type HttpHealthResourceModel struct {
	URL                  types.String `tfsdk:"url"`
	Id                   types.String `tfsdk:"id"`
	Method               types.String `tfsdk:"method"`
	Timeout              types.Int64  `tfsdk:"timeout"`
	RequestTimeout       types.Int64  `tfsdk:"request_timeout"`
	Interval             types.Int64  `tfsdk:"interval"`
	StatusCode           types.String `tfsdk:"status_code"`
	ConsecutiveSuccesses types.Int64  `tfsdk:"consecutive_successes"`
	Headers              types.Map    `tfsdk:"headers"`
	IgnoreFailure        types.Bool   `tfsdk:"create_anyway_on_check_failure"`
	Passed               types.Bool   `tfsdk:"passed"`
	RequestBody          types.String `tfsdk:"request_body"`
	ResultBody           types.String `tfsdk:"result_body"`
	CABundle             types.String `tfsdk:"ca_bundle"`
	InsecureTLS          types.Bool   `tfsdk:"insecure_tls"`
	Keepers              types.Map    `tfsdk:"keepers"`
}

func (r *HttpHealthResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_http_health"
}

func (r *HttpHealthResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HttpHealthResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue(uuid.NewString())

	r.HealthCheck(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)

}

func (r *HttpHealthResource) HealthCheck(ctx context.Context, data *HttpHealthResourceModel, diag *diag.Diagnostics) {
	data.Passed = types.BoolValue(false)
	endpoint, err := url.Parse(data.URL.ValueString())
	if err != nil {
		diag.AddError("Client Error", fmt.Sprintf("Unable to parse url %q, got error %s", data.URL.ValueString(), err))
		return
	}

	var checkCode func(int) bool
	if data.StatusCode.IsNull() {
		checkCode = func(c int) bool { return c < 400 }
	} else {
		v, err := strconv.Atoi(data.StatusCode.ValueString())
		if err != nil {
			diag.AddError("Error", fmt.Sprintf("Unable to parse status code pattern %s", err))
		}
		checkCode = func(c int) bool { return c == v }
	}

	// normalize headers
	headers := make(map[string][]string)
	if !data.Headers.IsNull() {
		tmp := make(map[string]string)
		diag.Append(data.Headers.ElementsAs(ctx, &tmp, false)...)
		if diag.HasError() {
			return
		}

		for k, v := range tmp {
			headers[k] = []string{v}
		}
	}

	window := helpers.RetryWindow{
		Timeout:              time.Duration(data.Timeout.ValueInt64()) * time.Millisecond,
		Interval:             time.Duration(data.Interval.ValueInt64()) * time.Millisecond,
		ConsecutiveSuccesses: int(data.ConsecutiveSuccesses.ValueInt64()),
	}
	data.ResultBody = types.StringValue("")

	if !data.CABundle.IsNull() && data.InsecureTLS.ValueBool() {
		diag.AddError("Conflicting configuration", "You cannot specify both custom CA and insecure TLS. Please use only one of them.")
	}
	tlsConfig := &tls.Config{}
	if !data.CABundle.IsNull() {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM([]byte(data.CABundle.ValueString())); !ok {
			diag.AddError("Building CA cert pool", err.Error())
		}
		tlsConfig.RootCAs = caCertPool
	}
	tlsConfig.InsecureSkipVerify = data.InsecureTLS.ValueBool()

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: time.Duration(data.RequestTimeout.ValueInt64()) * time.Millisecond,
	}

	tflog.Debug(ctx, fmt.Sprintf("Starting HTTP health check. Overall timeout: %d ms, request timeout: %d ms", data.Timeout.ValueInt64(), data.RequestTimeout.ValueInt64()))
	for h, v := range headers {
		tflog.Debug(ctx, fmt.Sprintf("%s: %s", h, v))
	}

	result := window.Do(func(attempt int, successes int) bool {
		if successes != 0 {
			tflog.Trace(ctx, fmt.Sprintf("SUCCESS [%d/%d] http %s %s", successes, data.ConsecutiveSuccesses.ValueInt64(), data.Method.ValueString(), endpoint))
		} else {
			tflog.Trace(ctx, fmt.Sprintf("ATTEMPT #%d http %s %s", attempt, data.Method.ValueString(), endpoint))
		}

		httpResponse, err := client.Do(&http.Request{
			URL:    endpoint,
			Method: data.Method.ValueString(),
			Header: headers,
			Body:   io.NopCloser(strings.NewReader(data.RequestBody.ValueString())),
		})
		if err != nil {
			diag.AddWarning("Error connecting to healthcheck endpoint", fmt.Sprintf("%s", err))
			return false
		}

		success := checkCode(httpResponse.StatusCode)
		if success {
			body, err := io.ReadAll(httpResponse.Body)
			if err != nil {
				diag.AddWarning("Error reading response body", fmt.Sprintf("%s", err))
				data.ResultBody = types.StringValue("")
			} else {
				data.ResultBody = types.StringValue(string(body))
			}
		}
		return success
	})

	switch result {
	case helpers.Success:
		data.Passed = types.BoolValue(true)
	case helpers.TimeoutExceeded:
		diag.AddWarning("Timeout exceeded", fmt.Sprintf("Timeout of %d milliseconds exceeded", data.Timeout.ValueInt64()))
		if !data.IgnoreFailure.ValueBool() {
			diag.AddError("Check failed", "The check did not pass and create_anyway_on_check_failure is false")
			return
		}
	}

}

func (r *HttpHealthResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HttpHealthResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *HttpHealthResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HttpHealthResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.HealthCheck(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HttpHealthResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *HttpHealthResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
