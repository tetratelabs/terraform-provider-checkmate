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
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/tetratelabs/terraform-provider-checkmate/pkg/helpers"
	"github.com/tetratelabs/terraform-provider-checkmate/pkg/modifiers"
)

var _ resource.Resource = &TCPEchoResource{}
var _ resource.ResourceWithImportState = &TCPEchoResource{}

type TCPEchoResource struct{}

// Schema implements resource.Resource
func (*TCPEchoResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "TCP Echo",

		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "The hostname where to send the TCP echo request to",
				Required:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "The port of the hostname where to send the TCP echo request",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 65535),
				},
			},
			"message": schema.StringAttribute{
				MarkdownDescription: "The message to send in the echo request",
				Required:            true,
			},
			"expected_message": schema.StringAttribute{
				MarkdownDescription: "The message expected to be included in the echo response",
				Required:            false,
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"persistent_response_regex": schema.StringAttribute{
				MarkdownDescription: `A regex pattern that the response need to match in every attempt to be considered successful.
  If not provided, the response is not checked.

  If using multiple attempts, this regex will be evaulated against the response text. For every susequent attempt, the regex
  will be evaluated against the response text and compared against the first obtained value. The check will be deemed successful
  if the regex matches the response text in every attempt. A single response not matching such value will cause the check to fail.`,
				Required: false,
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
			"expect_write_failure": schema.BoolAttribute{
				MarkdownDescription: "Wether or not the check is expected to fail after successfully connecting to the target. If true, the check will be considered successful if it fails. Defaults to false.",
				Required:            false,
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "Overall timeout in milliseconds for the check before giving up, default 10000",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(10000)},
			},
			"connection_timeout": schema.Int64Attribute{
				MarkdownDescription: "The timeout for stablishing a new TCP connection in milliseconds",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(5000)},
			},
			"single_attempt_timeout": schema.Int64Attribute{
				MarkdownDescription: "Timeout for an individual attempt. If exceeded, the attempt will be considered failure and potentially retried. Default 5000ms",
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
			"consecutive_successes": schema.Int64Attribute{
				MarkdownDescription: "Number of consecutive successes required before the check is considered successful overall. Defaults to 1.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(1)},
			},
			"passed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the check passed",
			},
			"create_anyway_on_check_failure": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "If false, the resource will fail to create if the check does not pass. If true, the resource will be created anyway. Defaults to false.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"keepers": schema.MapAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Arbitrary map of string values that when changed will cause the check to run again.",
				Optional:            true,
			},
		}}
}

type TCPEchoResourceModel struct {
	Id                      types.String `tfsdk:"id"`
	Host                    types.String `tfsdk:"host"`
	Port                    types.Int64  `tfsdk:"port"`
	Message                 types.String `tfsdk:"message"`
	ExpectedMessage         types.String `tfsdk:"expected_message"`
	PersistentResponseRegex types.String `tfsdk:"persistent_response_regex"`
	ExpectWriteFailure      types.Bool   `tfsdk:"expect_write_failure"`
	ConnectionTimeout       types.Int64  `tfsdk:"connection_timeout"`
	SingleAttemptTimeout    types.Int64  `tfsdk:"single_attempt_timeout"`
	Timeout                 types.Int64  `tfsdk:"timeout"`
	Interval                types.Int64  `tfsdk:"interval"`
	ConsecutiveSuccesses    types.Int64  `tfsdk:"consecutive_successes"`
	IgnoreFailure           types.Bool   `tfsdk:"create_anyway_on_check_failure"`
	Passed                  types.Bool   `tfsdk:"passed"`
	Keepers                 types.Map    `tfsdk:"keepers"`
}

// ImportState implements resource.ResourceWithImportState
func (*TCPEchoResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *TCPEchoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TCPEchoResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue(uuid.NewString())

	r.TCPEcho(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *TCPEchoResource) TCPEcho(ctx context.Context, data *TCPEchoResourceModel, diag *diag.Diagnostics) {
	if !data.ExpectWriteFailure.ValueBool() && data.ExpectedMessage.ValueString() == "" {
		tflog.Error(ctx, "expected_message is required when expect_failure is false")
		return
	}
	if data.ExpectedMessage.ValueString() != "" && data.ExpectWriteFailure.ValueBool() {
		tflog.Warn(ctx, "expected_message is ignored when expect_failure is true")
	}

	data.Passed = types.BoolValue(false)

	window := helpers.RetryWindow{
		Context:              ctx,
		Timeout:              time.Duration(data.Timeout.ValueInt64()) * time.Millisecond,
		Interval:             time.Duration(data.Interval.ValueInt64()) * time.Millisecond,
		ConsecutiveSuccesses: int(data.ConsecutiveSuccesses.ValueInt64()),
	}

	firstAttemptRegexValue := ""
	regexValueStoredAttempt := 0
	var persistentResponseRegex *regexp.Regexp
	var err error
	if data.PersistentResponseRegex.ValueString() != "" {
		persistentResponseRegex, err = regexp.Compile(data.PersistentResponseRegex.ValueString())
		if err != nil {
			tflog.Error(ctx, fmt.Sprintf("could not compile regex %q: %v", data.PersistentResponseRegex.ValueString(), err.Error()))
			diag.AddError("Invalid regex", fmt.Sprintf("Could not compile regex %q: %v", data.PersistentResponseRegex.ValueString(), err.Error()))
			return
		}
	}

	result := window.Do(func(attempt int, success int) bool {
		exepctFailure := data.ExpectWriteFailure.ValueBool()
		destStr := data.Host.ValueString() + ":" + strconv.Itoa(int(data.Port.ValueInt64()))

		d := net.Dialer{Timeout: time.Duration(data.ConnectionTimeout.ValueInt64()) * time.Millisecond}
		conn, err := d.Dial("tcp", destStr)
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("dial %q failed: %v", destStr, err.Error()))
			return false
		}
		defer conn.Close()

		_, err = conn.Write([]byte(data.Message.ValueString() + "\n"))
		if err != nil {
			tflog.Warn(ctx, fmt.Sprintf("write to server failed: %v", err.Error()))
			return false
		}

		deadlineDuration := time.Millisecond * time.Duration(data.SingleAttemptTimeout.ValueInt64())
		err = conn.SetDeadline(time.Now().Add(deadlineDuration))
		if err != nil && !exepctFailure {
			tflog.Warn(ctx, fmt.Sprintf("could not set connection deadline: %v", err.Error()))
			return false
		}

		reply := make([]byte, 1024)
		_, err = conn.Read(reply)
		if err != nil {
			if exepctFailure {
				// We expected this
				return true
			}
			tflog.Warn(ctx, fmt.Sprintf("read from server failed: %v", err.Error()))
			return false
		}
		// At this point, if we expect failure, we can just return the check failed,
		// as we were expecting it to fail
		if exepctFailure {
			return false
		}

		// remove null char from response
		reply = bytes.Trim(reply, "\x00")

		if persistentResponseRegex != nil {
			limits := persistentResponseRegex.FindStringIndex(string(reply))
			if limits == nil {
				tflog.Warn(ctx, fmt.Sprintf("Got response %q, which does not match regex %q", string(reply), data.PersistentResponseRegex.ValueString()))
				diag.AddWarning("Check failed", fmt.Sprintf("Got response %q, which does not match regex %q", string(reply), data.PersistentResponseRegex.ValueString()))
				return false
			}
			result := string(reply)[limits[0]:limits[1]]
			// result := persistentResponseRegex.FindString(string(reply))
			tflog.Info(ctx, fmt.Sprintf("Result: %s", result))

			// Avoid comparison on first attempt
			if regexValueStoredAttempt == 0 {
				firstAttemptRegexValue = result
				regexValueStoredAttempt = attempt
				return true
			}
			if firstAttemptRegexValue != result {
				tflog.Warn(ctx, fmt.Sprintf("Got response %q, which does not match previous attempt %q", result, firstAttemptRegexValue))
				diag.AddWarning("Check failed", fmt.Sprintf("Got response %q, which does not match previous attempt %q", result, firstAttemptRegexValue))
				return false
			}
		}

		if !strings.Contains(string(reply), data.ExpectedMessage.ValueString()) {
			tflog.Warn(ctx, fmt.Sprintf("Got response %q, which does not include expected message %q", string(reply), data.ExpectedMessage.ValueString()))
			diag.AddWarning("Check failed", fmt.Sprintf("Got response %q, which does not include expected message %q", string(reply), data.ExpectedMessage.ValueString()))
			return false
		}

		return true
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

// Delete implements resource.Resource
func (*TCPEchoResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *TCPEchoResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

// Metadata implements resource.Resource
func (*TCPEchoResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tcp_echo"
}

// Read implements resource.Resource
func (*TCPEchoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *TCPEchoResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *TCPEchoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *TCPEchoResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.TCPEcho(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewTCPEchoResource() resource.Resource {
	return &TCPEchoResource{}
}
