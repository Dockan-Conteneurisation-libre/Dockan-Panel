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
- run an image with a name and optional port
- run `dockan compose up`, `down`, `redeploy`, and `health` for a chosen `dockan.yml`
- manage Portainer-style stacks from the UI by saving `dockan.yml` files, then deploying, stopping, redeploying, and checking health
- import required stack images from a local Dockan registry folder before deployment

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

Default token from `dockan.yml`:

```text
dockan
```

This compose file uses `isolation: none` because the panel is an admin UI: it
must call the host `dockan` CLI and manage the host Dockan containers.

Stacks and backups are stored in the persistent `dockan-panel-data` volume.

On the Stacks page, fill `Required images` and `Registry folder`, then click
`Import Required Images`. This runs `dockan pull <image> <registry-folder>` for
each image. If the registry folder is empty, Dockan uses its default local
registry.

### Direct PHP

You can also run it directly with PHP. Set a token first:

```bash
cd /path/to/Dockan-Panel
export DOCKAN_UI_TOKEN="change-me"
php -S 127.0.0.1:9090 index.php
```

Open:

```text
http://127.0.0.1:9090
```

Use the token from `DOCKAN_UI_TOKEN` to log in.

If `dockan` is installed in `~/.local/bin`, the panel adds that directory to
`PATH` automatically.

## Notes

This UI executes the local `dockan` CLI as the current Linux user.

Keep it bound to `127.0.0.1` unless you put it behind proper authentication, HTTPS, and firewall rules.
