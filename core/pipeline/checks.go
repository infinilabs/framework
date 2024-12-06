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

// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package pipeline

import (
	"fmt"
	"github.com/rubyniu105/framework/core/config"
)

func ProcessorConfigChecked(
	constr ProcessorConstructor,
	checks ...func(*config.Config) error,
) ProcessorConstructor {
	validator := checkAll(checks...)
	return func(cfg *config.Config) (Processor, error) {
		err := validator(cfg)
		if err != nil {
			return nil, fmt.Errorf("%v in %v", err.Error(), cfg.Path())
		}

		return constr(cfg)
	}
}

func FilterConfigChecked(
	constr FilterConstructor,
	checks ...func(*config.Config) error,
) FilterConstructor {
	validator := checkAll(checks...)
	return func(cfg *config.Config) (Filter, error) {
		err := validator(cfg)
		if err != nil {
			return nil, fmt.Errorf("%v in %v", err.Error(), cfg.Path())
		}

		return constr(cfg)
	}
}

func checkAll(checks ...func(config *config.Config) error) func(*config.Config) error {
	return func(c *config.Config) error {
		for _, check := range checks {
			if err := check(c); err != nil {
				return err
			}
		}
		return nil
	}
}

// RequireFields checks that the required fields are present in the configuration.
func RequireFields(fields ...string) func(*config.Config) error {
	return func(cfg *config.Config) error {
		for _, field := range fields {
			if !cfg.HasField(field) {
				return fmt.Errorf("missing %v option", field)
			}
		}
		return nil
	}
}

// AllowedFields checks that only allowed fields are used in the configuration.
func AllowedFields(fields ...string) func(*config.Config) error {
	return func(cfg *config.Config) error {
		for _, field := range cfg.GetFields() {
			found := false
			for _, allowed := range fields {
				if field == allowed {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("unexpected %v option", field)
			}
		}
		return nil
	}
}

// MutuallyExclusiveRequiredFields checks that only one of the given
// fields is used at the same time. It is an error for none of the fields to be
// present.
func MutuallyExclusiveRequiredFields(fields ...string) func(*config.Config) error {
	return func(cfg *config.Config) error {
		var foundField string
		for _, field := range cfg.GetFields() {
			for _, f := range fields {
				if field == f {
					if len(foundField) == 0 {
						foundField = field
					} else {
						return fmt.Errorf("field %s and %s are mutually exclusive", foundField, field)
					}
				}
			}
		}

		if len(foundField) == 0 {
			return fmt.Errorf("missing option, select one from %v", fields)
		}
		return nil
	}
}
