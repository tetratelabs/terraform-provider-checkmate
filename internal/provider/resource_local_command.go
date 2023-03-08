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
	"os/exec"
	"time"

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

var _ resource.Resource = &LocalCommandResource{}
var _ resource.ResourceWithImportState = &LocalCommandResource{}

type LocalCommandResource struct{}

// Schema implements resource.Resource
func (*LocalCommandResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Local Command",

		Attributes: map[string]schema.Attribute{
			"command": schema.StringAttribute{
				MarkdownDescription: "The command to run (passed to `sh -c`)",
				Required:            true,
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "Overall timeout in milliseconds for the check before giving up, default 5000",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(5000)},
			},
			"retries": schema.Int64Attribute{
				MarkdownDescription: "Max number of times to retry a failure. Default 5",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(5)},
			},
			"interval": schema.Int64Attribute{
				MarkdownDescription: "Interval in milliseconds between attemps. Default 200",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(200)},
			},
			"consecutive_successes": schema.Int64Attribute{
				MarkdownDescription: "Number of consecutive successes required before the check is considered successful overall. Defaults to 1.\nIf there are fewer retries remaining than this number, the check will fail immediately",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(1)},
			},
			"working_directory": schema.StringAttribute{
				MarkdownDescription: "Working directory where the command will be run. Defaults to the current working directory",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{modifiers.DefaultString(".")},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		}}
}

type LocalCommandResourceModel struct {
	Command              types.String `tfsdk:"command"`
	Timeout              types.Int64  `tfsdk:"timeout"`
	Retries              types.Int64  `tfsdk:"retries"`
	Interval             types.Int64  `tfsdk:"interval"`
	ConsecutiveSuccesses types.Int64  `tfsdk:"consecutive_successes"`
	WorkDir              types.String `tfsdk:"working_directory"`
	Id                   types.String `tfsdk:"id"`
}

// ImportState implements resource.ResourceWithImportState
func (*LocalCommandResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *LocalCommandResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *LocalCommandResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.RunCommand(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LocalCommandResource) RunCommand(ctx context.Context, data *LocalCommandResourceModel, diag *diag.Diagnostics) {

	window := helpers.RetryWindow{
		MaxRetries:           int(data.Retries.ValueInt64()),
		Timeout:              time.Duration(data.Timeout.ValueInt64()) * time.Millisecond,
		Interval:             time.Duration(data.Interval.ValueInt64()) * time.Millisecond,
		ConsecutiveSuccesses: int(data.ConsecutiveSuccesses.ValueInt64()),
	}

	tflog.Error(ctx, "this is a log message of confusion")
	result := window.Do(func() bool {
		tflog.Debug(ctx, "i'm inside the function orange")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd := exec.Command("sh", "-c", data.Command.ValueString())
		cmd.Dir = data.WorkDir.ValueString()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			diag.AddWarning("Error running command", fmt.Sprintf("%s", err))
			tflog.Error(ctx, "why doesnt this happen")
			return false
		}
		tflog.Debug(ctx, fmt.Sprintf("Command stdout: %s", stdout.String()))
		tflog.Debug(ctx, fmt.Sprintf("Command stdout: %s", stderr.String()))
		return true
	})

	switch result {
	case helpers.Success:
		break
	case helpers.TimeoutExceeded:
		diag.AddError("Timeout exceeded", fmt.Sprintf("Timeout of %d milliseconds exceeded", data.Timeout.ValueInt64()))
		return
	case helpers.RetriesExceeded:
		diag.AddError("Retries exceeded", fmt.Sprintf("All %d attempts failed", data.Retries.ValueInt64()))
		return
	}

	data.Id = types.StringValue("example-id")
	tflog.Trace(ctx, "created resource")

}

// Delete implements resource.Resource
func (*LocalCommandResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *LocalCommandResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

// Metadata implements resource.Resource
func (*LocalCommandResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_local_command"
}

// Read implements resource.Resource
func (*LocalCommandResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *LocalCommandResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *LocalCommandResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *LocalCommandResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.RunCommand(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewLocalCommandResource() resource.Resource {
	return &LocalCommandResource{}
}
