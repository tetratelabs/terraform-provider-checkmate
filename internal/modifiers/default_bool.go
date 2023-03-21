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

var _ planmodifier.Bool = defaultBoolModifier{}

func DefaultBool(def bool) defaultBoolModifier {
	return defaultBoolModifier{Default: types.BoolValue(def)}
}

func NullableBool() defaultBoolModifier {
	return defaultBoolModifier{Default: types.BoolNull()}
}

type defaultBoolModifier struct {
	Default types.Bool
}

// PlanModifyBool implements planmodifier.Bool
func (m defaultBoolModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}

	resp.PlanValue = m.Default
}

func (m defaultBoolModifier) String() string {
	return m.Default.String()
}

func (m defaultBoolModifier) Description(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m)
}

func (m defaultBoolModifier) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m)
}
