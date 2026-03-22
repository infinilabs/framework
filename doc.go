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

/*
Package framework provides the core application lifecycle management for
building enterprise applications with the INFINI Framework.

The framework handles application initialization, configuration loading,
service management (install/uninstall/start/stop), signal handling,
graceful shutdown, and daemon mode support. It integrates with the core
subsystems including module registration, pipeline processing, task
scheduling, and statistics collection.

# Quick Start

Create a new application using [NewApp], initialize it, set up lifecycle
hooks, and run:

	app := framework.NewApp("myapp", "My Application", "1.0", "", "", "", "", "", "")
	app.Init(nil)
	if app.Setup(setupFunc, startFunc, stopFunc) {
		app.Run()
	}
	app.Shutdown()

The Setup function receives three callbacks: setup for one-time
initialization, start for launching services, and stop for cleanup on
shutdown.

# Service Management

The application can be managed as a system service using the -service flag:

	myapp -service install    # Install as system service
	myapp -service start      # Start the service
	myapp -service stop       # Stop the service
	myapp -service uninstall  # Remove the service

# Configuration

Configuration is loaded from a YAML file (default: <appname>.yml) and
supports environment variable interpolation and keystore-based secret
management. Use the -config flag to specify a custom config file path.
*/
package framework
