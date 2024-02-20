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
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/tetratelabs/terraform-provider-checkmate/pkg/healthcheck"
	"github.com/tetratelabs/terraform-provider-checkmate/pkg/modifiers"
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
				MarkdownDescription: "Status Code to expect. Can be a comma seperated list of ranges like '100-200,500'. Default 200",
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
			"jsonpath": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional JSONPath expression (same syntax as kubectl jsonpath output) to apply to the result body. If the expression matches, the check will pass.",
			},
			"json_value": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional regular expression to apply to the result of the JSONPath expression. If the expression matches, the check will pass.",
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
	JSONPath             types.String `tfsdk:"jsonpath"`
	JSONValue            types.String `tfsdk:"json_value"`
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
	var tmp map[string]string
	if !data.Headers.IsNull() {
		diag.Append(data.Headers.ElementsAs(ctx, &tmp, false)...)
	}
	args := healthcheck.HttpHealthArgs{
		URL:                  data.URL.ValueString(),
		Method:               data.Method.ValueString(),
		Timeout:              data.Timeout.ValueInt64(),
		RequestTimeout:       data.RequestTimeout.ValueInt64(),
		Interval:             data.Interval.ValueInt64(),
		StatusCode:           data.StatusCode.ValueString(),
		ConsecutiveSuccesses: data.ConsecutiveSuccesses.ValueInt64(),
		Headers:              tmp,
		IgnoreFailure:        data.IgnoreFailure.ValueBool(),
		RequestBody:          data.RequestBody.ValueString(),
		CABundle:             data.CABundle.ValueString(),
		InsecureTLS:          data.InsecureTLS.ValueBool(),
		JSONPath:             data.JSONPath.ValueString(),
		JSONValue:            data.JSONValue.ValueString(),
	}

	err := healthcheck.HealthCheck(ctx, &args, diag)
	if err != nil {
		diag.AddError("Health Check Error", fmt.Sprintf("Error during health check: %s", err))
	}

	data.Passed = types.BoolValue(args.Passed)
	data.ResultBody = types.StringValue(args.ResultBody)
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

func checkStatusCode(pattern string, code int, diag *diag.Diagnostics) bool {
	ranges := strings.Split(pattern, ",")
	for _, r := range ranges {
		bounds := strings.Split(r, "-")
		if len(bounds) == 2 {
			left, err := strconv.Atoi(bounds[0])
			if err != nil {
				diag.AddError("Bad status code pattern", fmt.Sprintf("Can't convert %s to integer. %s", bounds[0], err))
				return false
			}
			right, err := strconv.Atoi(bounds[1])
			if err != nil {
				diag.AddError("Bad status code pattern", fmt.Sprintf("Can't convert %s to integer. %s", bounds[1], err))
				return false
			}
			if left > right {
				diag.AddError("Bad status code pattern", fmt.Sprintf("Left bound %d is greater than right bound %d", left, right))
				return false
			}
			if left <= code && right >= code {
				return true
			}
		} else if len(bounds) == 1 {
			val, err := strconv.Atoi(bounds[0])
			if err != nil {
				diag.AddError("Bad status code pattern", fmt.Sprintf("Can't convert %s to integer. %s", bounds[0], err))
				return false
			}
			if val == code {
				return true
			}
		} else {
			diag.AddError("Bad status code pattern", "Too many dashes in range pattern")
			return false
		}
	}
	return false
}
