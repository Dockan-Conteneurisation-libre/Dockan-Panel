# Dockan PHP UI

A small local web interface for Dockan, inspired by Portainer but intentionally simpler.

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
cd Dockan-Panel
export DOCKAN_UI_TOKEN="change-me"
php -S 127.0.0.1:9090 index.php
```

Open:

```text
http://127.0.0.1:9090
```

Use the token from `DOCKAN_UI_TOKEN` to log in.

## Notes

This UI executes the local `dockan` CLI as the current Linux user.

Keep it bound to `127.0.0.1` unless you put it behind proper authentication, HTTPS, and firewall rules.
