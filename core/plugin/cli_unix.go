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
