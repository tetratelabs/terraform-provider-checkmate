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

package modifiers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ planmodifier.Int64 = defaultInt64Modifier{}

func DefaultInt64(def int64) defaultInt64Modifier {
	return defaultInt64Modifier{Default: types.Int64Value(def)}
}

type defaultInt64Modifier struct {
	Default types.Int64
}

// PlanModifyInt64 implements planmodifier.Int64
func (m defaultInt64Modifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() {
		return
	}

	resp.PlanValue = m.Default
}

func (m defaultInt64Modifier) String() string {
	return m.Default.String()
}

func (m defaultInt64Modifier) Description(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m)
}

func (m defaultInt64Modifier) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m)
}
