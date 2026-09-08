# Linux desktop package

Extract the archive, then run:

```sh
bash Caspian/install-desktop.sh
```

The installer requests administrator access. It installs the existing backend
services, the Flutter application in `/opt/caspian`, and an applications menu
entry. Existing settings remain available after an upgrade.

To remove both the services and desktop application, run:

```sh
bash /opt/caspian/uninstall-desktop.sh
```

The existing uninstaller asks whether to keep saved settings and restores the
network first. Desktop files are removed only after that command succeeds.
Its options, including `--keep-state`, `--purge`, and `--dry-run`, are supported.
