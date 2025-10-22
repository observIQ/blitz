# Linux Packages: Bindplane Loader

This document explains what the Linux packages install and how to run the service.

## What gets installed

- Binaries
  - `/usr/bin/bindplane-loader`
- Configuration
  - `/etc/bindplane-loader/config.yaml` (source: [`package/config.yaml`](../../package/config.yaml))
    - Owner: `bploader:bploader`
    - Mode: `0640`
- Systemd service
  - `/usr/lib/systemd/system/bindplane-loader.service` (source: [`package/bindplane-loader.service`](../../package/bindplane-loader.service))
    - Owner: `root:root`
    - Mode: `0640`
- Documentation and Licenses
  - `/usr/share/doc/bindplane-loader/LICENSE`
- Users and groups
  - A system user and group named `bploader` are created if they do not exist.
  - `/etc/bindplane-loader` is created and owned by `bploader:bploader`.
  - The `bploader` user is a non-login user with no shell (set to `nologin`). It cannot be used to log in via a terminal.

Notes:
- Package scripts reload systemd on install/upgrade/remove.
- The service is not started, enabled, or restarted by the package.

## Manage the service

Enable and start after install:

```bash
sudo systemctl enable bindplane-loader
sudo systemctl start bindplane-loader
sudo systemctl status bindplane-loader
```

Stop and disable:

```bash
sudo systemctl stop bindplane-loader
sudo systemctl disable bindplane-loader
```

## Configure

- Default config file: `/etc/bindplane-loader/config.yaml` (owned by `bploader`)
- Adjust permissions/ownership carefully if you change the run user.

## Uninstall

- Removing the package reloads systemd but does not start/stop or enable/disable services.
