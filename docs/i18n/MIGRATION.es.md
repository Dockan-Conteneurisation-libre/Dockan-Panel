# Migración del panel PHP al panel Go

[Esperanto](../MIGRATION.md) · [Français](MIGRATION.fr.md) · [English](MIGRATION.en.md) · [Español](MIGRATION.es.md)

El panel PHP v0.2.0 funciona con FrankenPHP; desde v0.2.1 el panel es un binario Go. Los datos siguen en el volumen `dockan-panel-data` y se reutilizan tal cual: cuentas (bcrypt `$2y$`), TOTP, llaves de acceso, stacks, copias y tienda.

El botón «Panel update» del panel PHP **no** hace la migración: solo copia archivos fuente, y el binario no está en las fuentes. Deja el panel PHP funcionando. Haz la migración a mano, en el servidor:

1. Haz una copia de los datos.
2. Descarga la versión y comprueba su suma SHA256 (para arm64, cambia `amd64`).
3. Descomprímela en `/srv/dockan-panel`: añade `dockan-panel` y reemplaza `Dockanfile` y `dockan.yml`.
4. Reinicia el servicio: `dockan compose up` reconstruye la imagen con el nuevo binario.

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

Después abre el panel e inicia sesión con tus cuentas habituales. Los antiguos `index.php` y `Caddyfile` de `/srv/dockan-panel` ya no se usan; puedes borrarlos.

## Volver a v0.2.0

```bash
curl -fsSL https://codeload.github.com/Dockan-Conteneurisation-libre/Dockan-Panel/tar.gz/v0.2.0 | sudo tar -xz --strip-components=1 -C /srv/dockan-panel
sudo systemctl restart dockan-dockan-panel.service
```
