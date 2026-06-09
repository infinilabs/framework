---
title: "Service Management"
weight: 28
---

# Service Management

Applications built on the INFINI Framework can integrate with the host service manager through the `-service` flag. The framework uses `github.com/kardianos/service` underneath, which maps service registration to the native platform implementation such as `systemd`, `launchd`, or the Windows Service Control Manager.

## Service Commands

The `-service` flag accepts the following commands:

| Command | Description |
|---------|-------------|
| `install` | Registers the current executable as a system service. |
| `uninstall` | Removes the registered service definition. |
| `start` | Starts the installed service. |
| `stop` | Stops the running service. |

Basic usage:

```bash
./myapp -service install
./myapp -service start
./myapp -service stop
./myapp -service uninstall
```

## Run the Service as a Specific User

Use `-service-user` during installation to specify the operating system account that should run the service:

```bash
sudo ./myapp -service install -service-user myapp
```

This is useful when the service should not run as `root`.

Notes:

- Registering or removing a system service usually still requires elevated privileges.
- The account passed to `-service-user` must already exist on the host.
- That account must be able to execute the binary and access the application's config, data, and log directories.
- If `-service-user` is omitted, the runtime account is left to the platform's default service behavior.

## Service Name

By default, the installed service name uses the application's lowercase name. You can override it with the `SERVICE_NAME` environment variable:

```bash
SERVICE_NAME=myapp-prod ./myapp -service install
```

This changes the internal service identifier used by the host service manager.

## Working Directory

The framework records the current working directory when the service is installed and uses it as the service working directory. A common deployment flow is:

```bash
cd /opt/myapp
sudo ./myapp -service install -service-user myapp
```

Install the service from the directory you want the application to run in.

## Configuration Notes

The framework still initializes its main config path before handling service control commands. In practice, this means:

- The main config file must exist when running `-service install`, `-service start`, `-service stop`, or `-service uninstall`.
- Using the default config filename in the service working directory keeps service installs predictable.
- Extra runtime flags are not automatically persisted into the installed service definition unless your application wires them into the service configuration explicitly.

## Typical Workflow

```bash
cd /opt/myapp
sudo ./myapp -service install -service-user myapp
sudo ./myapp -service start
sudo ./myapp -service stop
sudo ./myapp -service uninstall
```

For production deployments, make sure the target user can read the application configuration and write to the directories configured under `path.data` and `path.log`.