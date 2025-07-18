// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package orm

import (
	"context"
	"fmt"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/param"
)

type RefreshOption string

const (
	ctxKeyRefresh param.ParaKey = "refresh"

	RefreshFalse   RefreshOption = "false"    // default
	RefreshWaitFor RefreshOption = "wait_for" // safer async
	RefreshTrue    RefreshOption = "true"     // immediate + risky
)

func WithRefresh(ctx *orm.Context, opt RefreshOption) *orm.Context {
	ctx.SetValue(ctxKeyRefresh, opt)
	return ctx
}

func GetRefresh(ctx context.Context) RefreshOption {
	if val, ok := ctx.Value(ctxKeyRefresh).(RefreshOption); ok {
		return val
	}
	return RefreshFalse // default fallback
}

func ParseRefresh(input string) (RefreshOption, error) {
	switch input {
	case "", "false":
		return RefreshFalse, nil
	case "true":
		return RefreshTrue, nil
	case "wait_for":
		return RefreshWaitFor, nil
	default:
		return "", fmt.Errorf("invalid refresh option: %s", input)
	}
}
