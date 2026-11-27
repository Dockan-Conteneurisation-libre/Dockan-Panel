# Migrating from the PHP panel to the Go panel

[Esperanto](../MIGRATION.md) · [Français](MIGRATION.fr.md) · [English](MIGRATION.en.md) · [Español](MIGRATION.es.md)

The PHP panel v0.2.0 runs with FrankenPHP; since v0.2.1 the panel is a Go binary. The data stays in the `dockan-panel-data` volume and is reused as it is: accounts (bcrypt `$2y$`), TOTP, passkeys, stacks, backups and store.

The "Panel update" button of the PHP panel does **not** migrate: it only copies source files, and the binary is not in the sources. It leaves the PHP panel running. Migrate by hand, on the server:

1. Back up the data.
2. Download the release and check its SHA256 sum (for arm64, replace `amd64`).
3. Extract it into `/srv/dockan-panel`: it adds `dockan-panel` and replaces `Dockanfile` and `dockan.yml`.
4. Restart the service: `dockan compose up` rebuilds the image with the new binary.

```bash
sudo dockan volume backup dockan-panel-data /root/dockan-panel-data-v0.2.0.tar.gz
```

```bash
cd "$(mktemp -d)" && curl -fsSLO https://github.com/Dockan-Conteneurisation-libre/Dockan-Panel/releases/download/v0.2.1/dockan-panel-linux-amd64.tar.gz && curl -fsSLO https://github.com/Dockan-Conteneurisation-libre/Dockan-Panel/releases/download/v0.2.1/SHA256SUMS && sha256sum -c --ignore-missing SHA256SUMS
```

```bash
sudo tar -xzf dockan-panel-linux-amd64.tar.gz -C /srv --no-same-owner
```

```bash
sudo systemctl restart dockan-dockan-panel.service
```

Then open the panel and sign in with your usual accounts. The old `index.php` and `Caddyfile` in `/srv/dockan-panel` are no longer used; you can delete them.

## Going back to v0.2.0

```bash
curl -fsSL https://codeload.github.com/Dockan-Conteneurisation-libre/Dockan-Panel/tar.gz/v0.2.0 | sudo tar -xz --strip-components=1 -C /srv/dockan-panel
sudo systemctl restart dockan-dockan-panel.service
```
