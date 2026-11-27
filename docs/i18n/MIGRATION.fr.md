# Migration du panneau PHP vers le panneau Go

[Esperanto](../MIGRATION.md) · [Français](MIGRATION.fr.md) · [English](MIGRATION.en.md) · [Español](MIGRATION.es.md)

Le panneau PHP v0.2.0 tourne avec FrankenPHP ; depuis v0.2.1 le panneau est un binaire Go. Les données restent dans le volume `dockan-panel-data` et sont reprises telles quelles : comptes (bcrypt `$2y$`), TOTP, clés d'accès, stacks, sauvegardes et store.

Le bouton « Panel update » du panneau PHP ne fait **pas** la migration : il ne copie que des fichiers sources, et le binaire n'est pas dans les sources. Il laisse donc le panneau PHP en marche. Faites la migration à la main, sur le serveur :

1. Sauvegardez les données.
2. Téléchargez la version et vérifiez sa somme SHA256 (pour arm64, remplacez `amd64`).
3. Décompressez-la dans `/srv/dockan-panel` : elle ajoute `dockan-panel` et remplace `Dockanfile` et `dockan.yml`.
4. Redémarrez le service : `dockan compose up` reconstruit l'image avec le nouveau binaire.

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

Ouvrez ensuite le panneau et connectez-vous avec vos comptes habituels. Les anciens `index.php` et `Caddyfile` de `/srv/dockan-panel` ne servent plus ; vous pouvez les supprimer.

## Revenir à v0.2.0

```bash
curl -fsSL https://codeload.github.com/Dockan-Conteneurisation-libre/Dockan-Panel/tar.gz/v0.2.0 | sudo tar -xz --strip-components=1 -C /srv/dockan-panel
sudo systemctl restart dockan-dockan-panel.service
```
