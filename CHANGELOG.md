# Changelog

## v0.2.1

### Esperanto

- La panelo estas reverkita: unu statika Go-duumaĵo kun enkonstruita Vue 3-interfaco. PHP kaj FrankenPHP estas forigitaj.
- Nova interfaco en la stilo de la Dockan-retejo: flanka menuo, karto-paneloj, aga panelo kun la eligo de ĉiu komando, hela kaj malhela reĝimoj, poŝtelefono, kvar lingvoj (eo, fr, en, es).
- Kongrua kun Dockan v0.2.5: ujoj de ĉiuj deponejoj (`--scope all`), bildoj fiksitaj per `@sha256:`, privataj registrejoj, izolado `auto`.
- Interaga terminalo en la ujoj (xterm.js, pseŭdoterminalo en la servilo).
- La unua konto bezonas instalan kodon (protokolo aŭ `storage/setup-token`).
- Sekureco: atoma limigo de ensalutaj provoj (po kliento kaj po konto), limigitaj anonimaj seancoj, unufoja TOTP-kodo, pasvorto petata antaŭ malŝalti 2FA aŭ forigi aliroŝlosilon, aliaj seancoj fermitaj post ŝanĝo de pasvorto, alŝuto de retejo limigita per `os.Root`, neniu `-`-opcio enŝovebla en dockan.
- La datumoj de la PHP-panelo estas reuzataj senŝanĝe. Migrado: `docs/MIGRATION.md`.

### Français

- Le panneau est réécrit : un binaire Go statique avec l'interface Vue 3 intégrée. PHP et FrankenPHP sont retirés.
- Nouvelle interface dans le style du site Dockan : menu latéral, cartes, panneau d'activité avec la sortie de chaque commande, thèmes clair et sombre, mobile, quatre langues (eo, fr, en, es).
- Compatible avec Dockan v0.2.5 : conteneurs de tous les magasins (`--scope all`), images épinglées par `@sha256:`, registres privés, isolation `auto`.
- Terminal interactif dans les conteneurs (xterm.js, pseudo-terminal côté serveur).
- Le premier compte demande un code d'installation (journal ou `storage/setup-token`).
- Sécurité : limitation atomique des tentatives de connexion (par client et par compte), sessions anonymes plafonnées, code TOTP à usage unique, mot de passe demandé avant de désactiver la 2FA ou de supprimer une clé d'accès, autres sessions fermées après un changement de mot de passe, envoi de site confiné par `os.Root`, aucune option `-` injectable dans dockan.
- Les données du panneau PHP sont reprises telles quelles. Migration : `docs/i18n/MIGRATION.fr.md`.

### English

- The panel is rewritten: one static Go binary with the Vue 3 interface built in. PHP and FrankenPHP are removed.
- New interface in the style of the Dockan site: side menu, cards, activity panel with the output of every command, light and dark themes, mobile, four languages (eo, fr, en, es).
- Compatible with Dockan v0.2.5: containers of every store (`--scope all`), images pinned with `@sha256:`, private registries, `auto` isolation.
- Interactive terminal in containers (xterm.js, server-side pseudo-terminal).
- The first account needs a setup code (log or `storage/setup-token`).
- Security: atomic sign-in attempt limits (per client and per account), capped anonymous sessions, single-use TOTP codes, password asked before disabling 2FA or deleting a passkey, other sessions closed after a password change, site upload confined with `os.Root`, no `-` option can be injected into dockan.
- The PHP panel data is reused as it is. Migration: `docs/i18n/MIGRATION.en.md`.

### Español

- El panel se reescribe: un binario Go estático con la interfaz Vue 3 integrada. Se retiran PHP y FrankenPHP.
- Nueva interfaz con el estilo del sitio Dockan: menú lateral, tarjetas, panel de actividad con la salida de cada comando, temas claro y oscuro, móvil, cuatro idiomas (eo, fr, en, es).
- Compatible con Dockan v0.2.5: contenedores de todos los almacenes (`--scope all`), imágenes fijadas con `@sha256:`, registros privados, aislamiento `auto`.
- Terminal interactivo en los contenedores (xterm.js, pseudoterminal en el servidor).
- La primera cuenta pide un código de instalación (registro o `storage/setup-token`).
- Seguridad: límite atómico de intentos de inicio de sesión (por cliente y por cuenta), sesiones anónimas limitadas, código TOTP de un solo uso, contraseña pedida antes de desactivar la 2FA o eliminar una llave de acceso, otras sesiones cerradas tras un cambio de contraseña, subida de sitio confinada con `os.Root`, ninguna opción `-` inyectable en dockan.
- Los datos del panel PHP se reutilizan tal cual. Migración: `docs/i18n/MIGRATION.es.md`.

## v0.2.0

- OCI image pull support, `compose.yaml` support and store icons (PHP panel).
