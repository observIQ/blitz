# Linux Packages: Blitz

This document explains what the Linux packages install and how to run the service.

## What gets installed

- Binaries
  - `/usr/bin/blitz`
- Configuration
  - `/etc/blitz/config.yaml` (source: [`package/config.yaml`](../../package/config.yaml))
    - Owner: `blitz:blitz`
    - Mode: `0640`
- Systemd service
  - `/usr/lib/systemd/system/blitz.service` (source: [`package/blitz.service`](../../package/blitz.service))
    - Owner: `root:root`
    - Mode: `0640`
- Documentation and Licenses
  - `/usr/share/doc/blitz/LICENSE`
- Users and groups
  - A system user and group named `blitz` are created if they do not exist.
  - `/etc/blitz` is created and owned by `blitz:blitz`.
  - The `blitz` user is a non-login user with no shell (set to `nologin`). It cannot be used to log in via a terminal.

Notes:
- Package scripts reload systemd on install/upgrade/remove.
- The service is not started, enabled, or restarted by the package.

## Manage the service

Enable and start after install:

```bash
sudo systemctl enable blitz
sudo systemctl start blitz
sudo systemctl status blitz
```

Stop and disable:

```bash
sudo systemctl stop blitz
sudo systemctl disable blitz
```

## Configure

- Default config file: `/etc/blitz/config.yaml` (owned by `blitz`)
- Adjust permissions/ownership carefully if you change the run user.

## Uninstall

- Removing the package reloads systemd but does not start/stop or enable/disable services.
