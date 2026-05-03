# Dockan Panel

A small standalone PHP web interface for Dockan.

This project is separate from the Dockan CLI repository. It is meant to live in
its own repository and call the local `dockan` command installed on the machine.

It can:

- list containers, images, volumes, and networks
- show logs
- run healthchecks
- stop and remove containers
- remove images
- create volumes
- back up volumes to local `storage/backups`
- restore backups into a new empty volume
- create containers from local images with ports, volumes, environment variables, network, aliases, restart policy, healthcheck, CPU/RAM limits, GUI sockets, entrypoint, and command
- run `dockan compose up`, `down`, `redeploy`, and `health` for a chosen `dockan.yml`
- manage Portainer-style stacks from the UI by saving `dockan.yml` files, then deploying, stopping, redeploying, and checking health
- import required stack images from a local Dockan registry folder before deployment
- open a container detail page with actions, inspect output, logs, a live PTY terminal, and a one-shot exec fallback
- manage admin users with password login, optional authenticator-app 2FA, and passkeys
- install in the browser as a local PWA, with the Dockan logo and standalone window mode

## Start

### With Dockan

The panel can run as a Dockan app:

```bash
cd /path/to/Dockan-Panel
dockan compose up
```

Open:

```text
http://127.0.0.1:9090
```

On first launch, create the first admin account in the setup page. There is no
default password and no default token.

The panel is also installable as a PWA from browsers that support it. Open the
browser menu and choose the install/app shortcut option. PWA install works on
`localhost`, `127.0.0.1`, or HTTPS. The service worker is network-first and does
not cache authenticated admin pages, so container management still needs access
to the running local panel.

After login, open `Security` to add other admins, change passwords, enable 2FA,
or register passkeys. Passkeys require `localhost`, `127.0.0.1`, or HTTPS.

This compose file uses `isolation: none` because the panel is an admin UI: it
must call the host `dockan` CLI and manage the host Dockan containers.

Users, passkeys, stacks, and backups are stored in the persistent
`dockan-panel-data` volume.

The important files inside the panel are:

```text
/app/storage/auth-users.json
/app/storage/stacks/
/app/storage/backups/
```

On a normal user install, Dockan keeps the persistent volume on the host under:

```text
~/.local/share/dockan/volumes/dockan-panel-data
```

Do not remove this volume unless you want to reset the panel. Removing it
removes admin users, password hashes, 2FA secrets, passkeys, stacks, and panel
backups.

On the Stacks page, fill `Required images` and `Registry folder`, then click
`Import Required Images`. This runs `dockan pull <image> <registry-folder>` for
each image. If the registry folder is empty, Dockan uses its default local
registry.

### Direct PHP

You can also run it directly with PHP:

```bash
cd /path/to/Dockan-Panel
php -S 127.0.0.1:9090 index.php
```

Open:

```text
http://127.0.0.1:9090
```

Create the first admin account on the setup page.

If `dockan` is installed in `~/.local/bin`, the panel adds that directory to
`PATH` automatically.

## Persistent Storage

Dockan Panel keeps its data in `storage/` when it runs directly with PHP.

When it runs as a Dockan app, `storage/` is mounted from the
`dockan-panel-data` volume:

```yaml
volumes:
  - dockan-panel-data:/app/storage
```

The auth database is a local JSON file:

```text
storage/auth-users.json
```

It stores password hashes, not plain passwords. It also stores TOTP secrets and
passkey public keys. Treat this file like a secret and back it up safely.

## Backup And Restore

Backup the whole panel state:

```bash
dockan volume backup dockan-panel-data dockan-panel-backup.tar.gz
```

Restore it into a fresh volume:

```bash
dockan volume restore dockan-panel-data dockan-panel-backup.tar.gz
```

After restoring, redeploy the panel:

```bash
dockan compose redeploy
```

## Notes

This UI executes the local `dockan` CLI as the current Linux user.

The live terminal keeps a shell open through `dockan exec <container> sh -li`
and a local PTY helper. It uses `socat` when available, otherwise the
util-linux `script` command. The one-shot Quick Exec form is still available
for simple commands.

In the Containers page, click a container name to open its detail page. From
there you can run commands inside it, read logs, inspect metadata, stop it,
remove it, or run its healthcheck.

Keep it bound to `127.0.0.1` unless you put it behind proper authentication, HTTPS, and firewall rules.

Passkeys are verified server-side with the browser challenge and signature. They
are still browser-dependent: if your browser cannot expose the WebAuthn public
key during registration, use password plus 2FA instead.
