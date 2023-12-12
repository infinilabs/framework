// +build !windows

package plugin

import (
	goplugin "plugin"
)

func loadPlugins(path string) error {
	p, err := goplugin.Open(path)
	if err != nil {
		return err
	}

	sym, err := p.Lookup("Init")
	if err != nil {
		return err
	}

	sym.(func())()

	return nil
}
