# Migrado de la PHP-panelo al Go-panelo

[Esperanto](MIGRATION.md) · [Français](i18n/MIGRATION.fr.md) · [English](i18n/MIGRATION.en.md) · [Español](i18n/MIGRATION.es.md)

La PHP-panelo v0.2.0 kuras per FrankenPHP; ekde v0.2.1 la panelo estas unu Go-duumaĵo. La datumoj restas en la volumo `dockan-panel-data` kaj estas reuzataj senŝanĝe: kontoj (bcrypt `$2y$`), TOTP, aliroŝlosiloj, stakoj, savkopioj kaj vendejo.

La butono "Panel update" de la PHP-panelo **ne** faras la migradon: ĝi kopias nur fontajn dosierojn, kaj la duumaĵo mankas en la fonto. Ĝi do lasas la PHP-panelon funkcianta. Faru la migradon permane, en la servilo:

1. Savkopiu la datumojn.
2. Elŝutu la eldonon kaj kontrolu ĝian SHA256-sumon (por arm64, anstataŭigu `amd64`).
3. Malpaku ĝin en `/srv/dockan-panel`: ĝi aldonas `dockan-panel` kaj anstataŭigas `Dockanfile` kaj `dockan.yml`.
4. Restartigu la servon: `dockan compose up` rekonstruas la bildon kun la nova duumaĵo.

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

Poste malfermu la panelon kaj ensalutu per viaj kutimaj kontoj. La malnovaj `index.php` kaj `Caddyfile` en `/srv/dockan-panel` ne plu estas uzataj; vi povas forigi ilin.

## Reveni al v0.2.0

```bash
curl -fsSL https://codeload.github.com/Dockan-Conteneurisation-libre/Dockan-Panel/tar.gz/v0.2.0 | sudo tar -xz --strip-components=1 -C /srv/dockan-panel
sudo systemctl restart dockan-dockan-panel.service
```
