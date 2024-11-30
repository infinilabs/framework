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

// +build !windows

package plugin

import (
	"flag"
	log "github.com/cihub/seelog"
	"strings"
)

type pluginList struct {
	paths  []string
}

func (p *pluginList) String() string {
	return strings.Join(p.paths, ",")
}

func (p *pluginList) Set(v string) error {
	for _, path := range p.paths {
		if path == v {
			log.Warnf("%s is already a registered plugin", path)
			return nil
		}
	}
	p.paths = append(p.paths, v)
	return nil
}

var plugins = &pluginList{
}

func init() {
	flag.Var(plugins, "plugin", "load additional plugins")
}

func Initialize() error {
	for _, path := range plugins.paths {
		log.Infof("loading plugin: %v", path)
		if err := LoadPlugins(path); err != nil {
			return err
		}
	}

	return nil
}
