/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package reverse

import "infini.sh/framework/core/module"

type Module struct{}

func (m *Module) Name() string { return "reverse_channel" }
func (m *Module) Setup()       {}
func (m *Module) Start() error { Setup(); return nil }
func (m *Module) Stop() error  { return nil }

func init() {
	// After the managed bootstrap (100) so the manager token from
	// registration/exchange is likely already in the keystore.
	module.RegisterModuleWithPriority(&Module{}, 101)
}
