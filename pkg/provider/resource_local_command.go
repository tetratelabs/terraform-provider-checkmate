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
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/tetratelabs/terraform-provider-checkmate/pkg/helpers"
	"github.com/tetratelabs/terraform-provider-checkmate/pkg/modifiers"
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
				MarkdownDescription: "Overall timeout in milliseconds for the check before giving up, default 10000",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{modifiers.DefaultInt64(10000)},
			},
			"command_timeout": schema.Int64Attribute{
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
			"working_directory": schema.StringAttribute{
				MarkdownDescription: "Working directory where the command will be run. Defaults to the current working directory",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{modifiers.DefaultString(".")},
			},
			"env": schema.MapAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Map of environment variables to apply to the command. Inherits the parent environment",
				Optional:            true,
			},
			"create_file": schema.SingleNestedAttribute{
				MarkdownDescription: "Ensure a file exists with the following contents. The path to this file will be available in the env var CHECKMATE_FILEPATH",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"contents": schema.StringAttribute{
						MarkdownDescription: "Contents of the file to create",
						Required:            true,
					},
					"name": schema.StringAttribute{
						MarkdownDescription: "Name of the created file.",
						Required:            true,
					},
					"path": schema.StringAttribute{
						MarkdownDescription: "Path to the file that was created",
						Computed:            true,
					},
					"use_working_dir": schema.BoolAttribute{
						MarkdownDescription: "If true, will use the working directory instead of a temporary directory. Defaults to false.",
						Optional:            true,
					},
					"create_directory": schema.BoolAttribute{
						MarkdownDescription: "Create the target directory if it doesn't exist. Defaults to false.",
						Optional:            true,
					},
				},
			},
			"stdout": schema.StringAttribute{
				MarkdownDescription: "Standard output of the command",
				Computed:            true,
			},
			"stderr": schema.StringAttribute{
				MarkdownDescription: "Standard error output of the command",
				Computed:            true,
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

type CreateFileModel struct {
	Contents            types.String `tfsdk:"contents"`
	Path                types.String `tfsdk:"path"`
	UseWorkingDirectory types.Bool   `tfsdk:"use_working_dir"`
	Name                types.String `tfsdk:"name"`
	CreateDirectory     types.Bool   `tfsdk:"create_directory"`
}

type LocalCommandResourceModel struct {
	Id                   types.String     `tfsdk:"id"`
	Command              types.String     `tfsdk:"command"`
	Timeout              types.Int64      `tfsdk:"timeout"`
	CommandTimeout       types.Int64      `tfsdk:"command_timeout"`
	Interval             types.Int64      `tfsdk:"interval"`
	ConsecutiveSuccesses types.Int64      `tfsdk:"consecutive_successes"`
	WorkDir              types.String     `tfsdk:"working_directory"`
	Stdout               types.String     `tfsdk:"stdout"`
	Stderr               types.String     `tfsdk:"stderr"`
	Env                  types.Map        `tfsdk:"env"`
	CreateFile           *CreateFileModel `tfsdk:"create_file"`
	IgnoreFailure        types.Bool       `tfsdk:"create_anyway_on_check_failure"`
	Passed               types.Bool       `tfsdk:"passed"`
	Keepers              types.Map        `tfsdk:"keepers"`
}

// ImportState implements resource.ResourceWithImportState
func (*LocalCommandResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, tfpath.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *LocalCommandResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LocalCommandResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue(uuid.NewString())

	r.EnsureFile(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r.RunCommand(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *LocalCommandResource) RunCommand(ctx context.Context, data *LocalCommandResourceModel, diag *diag.Diagnostics) {
	data.Passed = types.BoolValue(false)
	data.Stdout = types.StringNull()
	data.Stderr = types.StringNull()

	window := helpers.RetryWindow{
		Context:              ctx,
		Timeout:              time.Duration(data.Timeout.ValueInt64()) * time.Millisecond,
		Interval:             time.Duration(data.Interval.ValueInt64()) * time.Millisecond,
		ConsecutiveSuccesses: int(data.ConsecutiveSuccesses.ValueInt64()),
	}

	envMap := make(map[string]string)
	if !data.Env.IsNull() {
		diag.Append(data.Env.ElementsAs(ctx, &envMap, false)...)
		if diag.HasError() {
			return
		}

	}

	if data.CreateFile != nil {
		abs, err := filepath.Abs(data.CreateFile.Path.ValueString())
		if err != nil {
			tflog.Error(ctx, fmt.Sprintf("Can't determine the absolute path of the file we created at %q error=%v", data.CreateFile.Path.ValueString(), err))
			diag.AddError("Can't get path to file", fmt.Sprintf("Can't determine the absolute path of the file we created at %q error=%v", data.CreateFile.Path.ValueString(), err))
			return
		}
		envMap["CHECKMATE_FILEPATH"] = abs
	}

	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	tflog.Debug(ctx, fmt.Sprintf("Command string: sh -c %s", data.Command.ValueString()))
	result := window.Do(func(attempt int, successes int) bool {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		commandContext, cancelFunc := context.WithTimeout(ctx, time.Duration(data.CommandTimeout.ValueInt64())*time.Millisecond)
		defer cancelFunc()

		cmd := exec.CommandContext(commandContext, "sh", "-c", data.Command.ValueString())
		cmd.Dir = data.WorkDir.ValueString()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), env...)

		err := cmd.Start()
		if err != nil {
			tflog.Trace(ctx, fmt.Sprintf("ATTEMPT #%d error starting command", attempt))
			tflog.Error(ctx, fmt.Sprintf("Error starting command %v", err))
			return false
		}
		err = cmd.Wait()
		if err != nil {
			tflog.Trace(ctx, fmt.Sprintf("ATTEMPT #%d exit_code=%d", attempt, err.(*exec.ExitError).ExitCode()))
			data.Stdout = types.StringValue(stdout.String())
			data.Stderr = types.StringValue(stderr.String())
			tflog.Debug(ctx, fmt.Sprintf("Command stdout: %s", stdout.String()))
			tflog.Debug(ctx, fmt.Sprintf("Command stdout: %s", stderr.String()))
			return false
		}
		tflog.Trace(ctx, fmt.Sprintf("SUCCESS [%d/%d]", successes, data.ConsecutiveSuccesses.ValueInt64()))
		data.Stdout = types.StringValue(stdout.String())
		data.Stderr = types.StringValue(stderr.String())
		tflog.Debug(ctx, fmt.Sprintf("Command stdout: %s", stdout.String()))
		tflog.Debug(ctx, fmt.Sprintf("Command stdout: %s", stderr.String()))
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

	r.EnsureFile(ctx, data, &resp.Diagnostics)
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

func (r *LocalCommandResource) EnsureFile(ctx context.Context, data *LocalCommandResourceModel, diag *diag.Diagnostics) {
	cf := data.CreateFile
	if cf == nil {
		return
	}

	var file *os.File
	var err error
	if cf.UseWorkingDirectory.ValueBool() {
		if cf.CreateDirectory.ValueBool() {
			dir := filepath.Join(data.WorkDir.ValueString(), filepath.Dir(cf.Name.ValueString()))
			err = os.MkdirAll(dir, 0750)
			if err != nil {
				diag.AddError("Error creating directory", fmt.Sprintf("Error creating directory %q. %v", dir, err))
				return
			}

		}
		file, err = os.CreateTemp(data.WorkDir.ValueString(), cf.Name.ValueString())
		if err != nil {
			diag.AddError("Error creating file", fmt.Sprintf("%s", err))
			return
		}
	} else {
		file, err = os.CreateTemp("", cf.Name.ValueString())
		if err != nil {
			diag.AddError("Error creating file", fmt.Sprintf("%s", err))
			return
		}
	}

	_, err = file.WriteString(cf.Contents.ValueString())
	if err != nil {
		diag.AddError("Error writing file", fmt.Sprintf("%s", err))
		return
	}

	cf.Path = types.StringValue(file.Name())
}
