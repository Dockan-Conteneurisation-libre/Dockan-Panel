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

## Start

Set a token first:

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
