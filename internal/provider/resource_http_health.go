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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
			"retries": schema.Int64Attribute{
				MarkdownDescription: "Max number of times to retry a failure. Exceeding this number will cause the check to fail even if timeout has not expired yet.\n Default 5.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(5)},
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
			"result_body": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Result body",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"passed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the check passed",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"ignore_failure": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "If set to true, the check will not be considered a failure when it does not pass",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

type HttpHealthResourceModel struct {
	URL                  types.String `tfsdk:"url"`
	Id                   types.String `tfsdk:"id"`
	Retries              types.Int64  `tfsdk:"retries"`
	Method               types.String `tfsdk:"method"`
	Timeout              types.Int64  `tfsdk:"timeout"`
	Interval             types.Int64  `tfsdk:"interval"`
	StatusCode           types.String `tfsdk:"status_code"`
	ConsecutiveSuccesses types.Int64  `tfsdk:"consecutive_successes"`
	Headers              types.Map    `tfsdk:"headers"`
	IgnoreFailure        types.Bool   `tfsdk:"ignore_failure"`
	Passed               types.Bool   `tfsdk:"passed"`
	ResultBody           types.String `tfsdk:"result_body"`
}

func (r *HttpHealthResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_http_health"
}

func (r *HttpHealthResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *HttpHealthResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.HealthCheck(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

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

		for k, v := range data.Headers.Elements() {
			headers[k] = []string{v.String()}
		}
	}

	window := helpers.RetryWindow{
		MaxRetries:           int(data.Retries.ValueInt64()),
		Timeout:              time.Duration(data.Timeout.ValueInt64()) * time.Millisecond,
		Interval:             time.Duration(data.Interval.ValueInt64()) * time.Millisecond,
		ConsecutiveSuccesses: int(data.ConsecutiveSuccesses.ValueInt64()),
	}

	client := http.DefaultClient
	result := window.Do(func() bool {
		httpResponse, err := client.Do(&http.Request{
			URL:    endpoint,
			Method: data.Method.ValueString(),
			Header: headers,
		})
		if err != nil {
			diag.AddWarning("Error connecting to healthcheck endpoint", fmt.Sprintf("%s", err))
			return false
		}

		success := checkCode(httpResponse.StatusCode)
		if success {
			body, err := ioutil.ReadAll(httpResponse.Body)
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
			diag.AddError("Check failed", "The check did not pass and ignore_error is false")
			return
		}
	case helpers.RetriesExceeded:
		diag.AddWarning("Retries exceeded", fmt.Sprintf("All %d attempts failed", data.Retries.ValueInt64()))
		if !data.IgnoreFailure.ValueBool() {
			diag.AddError("Check failed", "The check did not pass and ignore_error is false")
			return
		}
	}

	data.Id = types.StringValue("example-id")

}

func (r *HttpHealthResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *HttpHealthResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HttpHealthResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *HttpHealthResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.HealthCheck(ctx, data, &resp.Diagnostics)
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
