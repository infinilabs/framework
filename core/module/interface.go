/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package module

// Module defines system level module structure
type Module interface {
	Setup()
	Start() error
	Stop() error
	Name() string
}

////implement template
//type Module struct {
//}
//func (this *Module) Setup() {
//
//}
//func (this *Module) Start() error {
//	return nil
//}
//func (this *Module) Stop() error {
//	return nil
//}
//func (this *Module) Name() string {
//	return "NAME"
//}
