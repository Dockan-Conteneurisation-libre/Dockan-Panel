<p align="center">
  <img src="./dockan-logo.svg" alt="Dockan Panel logo" width="96">
</p>

<h1 align="center">Dockan Panel</h1>

<p align="center">
  A small standalone PHP web interface for Dockan.
</p>

<p align="center">
  <img alt="PHP 8.3+" src="https://img.shields.io/badge/PHP-8.3%2B-777BB4?logo=php&logoColor=white">
  <img alt="Dockan app" src="https://img.shields.io/badge/runs%20with-Dockan-176b48">
  <img alt="PWA ready" src="https://img.shields.io/badge/PWA-ready-0e4932">
  <img alt="Local first" src="https://img.shields.io/badge/local--first-admin%20panel-17201b">
</p>

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
- inspect Dockan, PHP, and FrankenPHP versions, preview dependency installs, install host packages/runtimes, and run Dockan updates
- open a container detail page with actions, inspect output, logs, a live PTY terminal, and a one-shot exec fallback
- manage admin users with password login, optional authenticator-app 2FA, and passkeys
- install in the browser as a local PWA, with the Dockan logo and standalone window mode

## Start

### With Dockan

The panel can run as a Dockan app. Install the `frankenphp` binary on the host
first; the Dockan image is intentionally `scratch` and calls that local runtime.

```bash
cd /path/to/Dockan-Panel
dockan compose up
```

Open:

```text
http://127.0.0.1:9090
```

For production, install the same Dockan app as a system service with Dockan:

```bash
sudo dockan service install -f /path/to/Dockan-Panel/dockan.yml --name dockan-panel
sudo systemctl daemon-reload
sudo systemctl enable --now dockan-dockan-panel.service
```

This keeps Dockan Panel managed by Dockan while starting it as a root/system
service. In that mode the `Packages` page can run host package installs and
runtime updates directly.

On a server, the default image runs FrankenPHP/Caddy and listens on all
interfaces:

```yaml
command: frankenphp run --config Caddyfile
```

That means the panel can also be reached from the private network with:

```text
http://SERVER_PRIVATE_IP:9090
```

For a local-only panel, override the bind address:

```yaml
env:
  - DOCKAN_BIN=dockan
  - PORT=9090
  - BIND=127.0.0.1
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

Open `Packages` to check the current Dockan version, preview dependency
profiles, install host dependencies, prepare runtimes such as FrankenPHP or
Node, and run release updates. For one-click system installs, run the panel as a
root/system service or grant passwordless permission for Dockan package actions.
When the panel is launched as a normal user, the `Packages` page marks system
automation as disabled and keeps `Preview`/`Show Command` available.

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

## Production Exposure

Dockan Panel is an admin UI. Do not expose it directly to the Internet over
plain HTTP.

The recommended production shape is:

```text
Internet -> HTTPS reverse proxy -> Dockan Panel
```

Pangolin, Caddy, Nginx, Traefik, or Cloudflare Tunnel can be used as the HTTPS
reverse proxy.

### Default Server Mode

By default, Dockan Panel uses the host `frankenphp` binary and listens on all
interfaces:

```yaml
command: frankenphp run --config Caddyfile
```

This makes it usable from another machine on the private network:

```text
http://192.168.x.x:9090
```

Use this when Pangolin or another reverse proxy is not installed on the same
machine as Dockan Panel.

### Pangolin On Another Machine

If Pangolin runs on another server, keep the default `BIND=0.0.0.0` setting and
redeploy:

```bash
dockan compose redeploy
```

From the Dockan server, verify both addresses:

```bash
curl http://127.0.0.1:9090
curl http://192.168.x.x:9090
```

In Pangolin, set the internal/backend URL to the Dockan server IP:

```text
http://192.168.x.x:9090
```

Replace `192.168.x.x` with the real private IP of the Dockan Panel server.

Protect port `9090` with a firewall so only the Pangolin server can reach it.

With `ufw`:

```bash
sudo ufw allow from PANGOLIN_IP to any port 9090 proto tcp
sudo ufw deny 9090/tcp
```

With `firewalld`:

```bash
sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="PANGOLIN_IP" port port="9090" protocol="tcp" accept'
sudo firewall-cmd --reload
```

Replace `PANGOLIN_IP` with the private IP of the Pangolin server.

### Pangolin On The Same Machine

If Pangolin runs on the same server as Dockan Panel, you can make the panel
local-only:

```yaml
env:
  - DOCKAN_BIN=dockan
  - PORT=9090
  - BIND=127.0.0.1
```

Then redeploy:

```bash
dockan compose redeploy
```

In Pangolin, set the internal/backend URL to:

```text
http://127.0.0.1:9090
```

Your public URL can then be something like:

```text
https://dockan.example.com
```

This is the safest setup when Pangolin and Dockan Panel are on the same
machine, because port `9090` is reachable only locally.

### Public HTTPS Notes

Use HTTPS for the public URL. Passkeys work on `localhost`, `127.0.0.1`, or
HTTPS. Browsers usually block passkeys on plain HTTP LAN addresses such as
`http://192.168.x.x:9090`.

Keep strong admin passwords, enable 2FA or passkeys, and restrict access with a
VPN, allowlist, or Pangolin access policy when possible.

### Direct PHP For Development

You can also run it directly with PHP during local development:

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

If it listens on `BIND=0.0.0.0`, protect port `9090` with a firewall and expose the
public access through HTTPS. If the reverse proxy runs on the same machine, set
`BIND=127.0.0.1` for the panel service.

Passkeys are verified server-side with the browser challenge and signature. They
are still browser-dependent: if your browser cannot expose the WebAuthn public
key during registration, use password plus 2FA instead.
# Dockan-Panel
