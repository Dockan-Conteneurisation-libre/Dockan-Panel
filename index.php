<?php
declare(strict_types=1);

session_start();

const APP_NAME = 'Dockan Panel';
const STORAGE_DIR = __DIR__ . '/storage';
const BACKUP_DIR = STORAGE_DIR . '/backups';
const STACKS_DIR = STORAGE_DIR . '/stacks';

ensure_storage();

if (($_GET['asset'] ?? '') === 'logo') {
    header('Content-Type: image/svg+xml');
    readfile(__DIR__ . '/dockan-logo.svg');
    exit;
}

$token = getenv('DOCKAN_UI_TOKEN') ?: '';
$dockan = getenv('DOCKAN_BIN') ?: 'dockan';
$flash = null;
$error = null;
$view = $_GET['view'] ?? 'dashboard';

if (isset($_POST['logout'])) {
    $_SESSION = [];
    session_destroy();
    header('Location: ' . self_url());
    exit;
}

if ($token === '') {
    render_page('Locked', locked_content(), false);
    exit;
}

if (!is_logged_in()) {
    if ($_SERVER['REQUEST_METHOD'] === 'POST' && ($_POST['token'] ?? '') !== '') {
        if (hash_equals($token, (string) $_POST['token'])) {
            $_SESSION['dockan_ui_auth'] = true;
            $_SESSION['csrf'] = bin2hex(random_bytes(24));
            header('Location: ' . self_url());
            exit;
        }
        $error = 'Invalid token.';
    }
    render_page('Login', login_content($error), false);
    exit;
}

if (empty($_SESSION['csrf'])) {
    $_SESSION['csrf'] = bin2hex(random_bytes(24));
}

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['action'])) {
    try {
        verify_csrf();
        $result = handle_action((string) $_POST['action'], $dockan);
        $flash = $result === '' ? 'Done.' : $result;
    } catch (Throwable $e) {
        $error = $e->getMessage();
    }
}

$content = match ($view) {
    'containers' => containers_content($dockan),
    'images' => images_content($dockan),
    'volumes' => volumes_content($dockan),
    'networks' => networks_content($dockan),
    'stacks' => stacks_content($dockan),
    'compose' => compose_content($dockan),
    'logs' => logs_content($dockan),
    default => dashboard_content($dockan),
};

render_page(page_title($view), $content, true, $flash, $error);

function handle_action(string $action, string $dockan): string
{
    return match ($action) {
        'stop-container' => command_text(run_dockan($dockan, ['stop', required_post('name')])),
        'remove-container' => command_text(run_dockan($dockan, ['rm', required_post('name')])),
        'health-container' => command_text(run_dockan($dockan, ['health', required_post('name')])),
        'remove-image' => command_text(run_dockan($dockan, ['rmi', required_post('tag')])),
        'create-volume' => command_text(run_dockan($dockan, ['volume', 'create', required_post('name')])),
        'remove-volume' => command_text(run_dockan($dockan, ['volume', 'rm', required_post('name')])),
        'backup-volume' => backup_volume($dockan, required_post('name')),
        'restore-volume' => restore_volume($dockan),
        'run-image' => run_image($dockan),
        'compose-up' => compose_action($dockan, 'up'),
        'compose-down' => compose_action($dockan, 'down'),
        'compose-redeploy' => compose_action($dockan, 'redeploy'),
        'compose-health' => compose_action($dockan, 'health'),
        'stack-save' => stack_save(),
        'stack-delete' => stack_delete(),
        'stack-up' => stack_compose_action($dockan, 'up'),
        'stack-down' => stack_compose_action($dockan, 'down'),
        'stack-redeploy' => stack_compose_action($dockan, 'redeploy'),
        'stack-health' => stack_compose_action($dockan, 'health'),
        'stack-import-required' => stack_import_required_images($dockan),
        default => throw new RuntimeException('Unknown action.'),
    };
}

function run_image(string $dockan): string
{
    $name = required_post('name');
    $image = required_post('image');
    $ports = trim((string) ($_POST['ports'] ?? ''));
    $args = ['run', '-d', '--name', $name];
    if ($ports !== '') {
        $args[] = '-p';
        $args[] = $ports;
    }
    $args[] = $image;
    return command_text(run_dockan($dockan, $args));
}

function compose_action(string $dockan, string $action): string
{
    $file = required_post('file');
    if (!is_file($file)) {
        throw new RuntimeException('dockan.yml not found.');
    }
    return command_text(run_dockan($dockan, ['compose', $action, '-f', $file]));
}

function stack_save(): string
{
    $name = clean_stack_name(required_post('name'));
    $yaml = trim((string) ($_POST['yaml'] ?? ''));
    $requiredImages = parse_image_list((string) ($_POST['required_images'] ?? ''));
    $registryDir = trim((string) ($_POST['registry_dir'] ?? ''));
    persist_stack($name, $yaml, $requiredImages, $registryDir);
    $_GET['stack'] = $name;
    return 'Stack saved: ' . $name;
}

function stack_delete(): string
{
    $name = clean_stack_name(required_post('stack'));
    $dir = stack_dir($name);
    if (!is_dir($dir)) {
        throw new RuntimeException('Stack not found.');
    }
    foreach ([stack_file($name), stack_required_images_file($name), stack_registry_file($name)] as $file) {
        if (is_file($file) && !unlink($file)) {
            throw new RuntimeException('Unable to delete stack file.');
        }
    }
    if (!rmdir($dir)) {
        throw new RuntimeException('Unable to delete stack directory.');
    }
    unset($_GET['stack']);
    return 'Stack deleted: ' . $name;
}

function stack_compose_action(string $dockan, string $action): string
{
    $name = clean_stack_name(required_post('stack'));
    $file = stack_file($name);
    if (!is_file($file)) {
        throw new RuntimeException('Stack not found.');
    }
    return command_text(run_dockan($dockan, ['compose', $action, '-f', $file]));
}

function stack_import_required_images(string $dockan): string
{
    $name = clean_stack_name(required_post('name'));
    $yaml = trim((string) ($_POST['yaml'] ?? ''));
    $requiredImages = parse_image_list((string) ($_POST['required_images'] ?? ''));
    $registryDir = trim((string) ($_POST['registry_dir'] ?? ''));
    if ($yaml !== '') {
        persist_stack($name, $yaml, $requiredImages, $registryDir);
    }
    if (!$requiredImages && is_file(stack_required_images_file($name))) {
        $requiredImages = parse_image_list((string) file_get_contents(stack_required_images_file($name)));
    }
    if (!$requiredImages && is_file(stack_file($name))) {
        $requiredImages = detect_stack_images((string) file_get_contents(stack_file($name)));
    }
    if (!$requiredImages) {
        throw new RuntimeException('No required images found.');
    }
    $output = [];
    foreach ($requiredImages as $image) {
        $args = ['pull', $image];
        if ($registryDir !== '') {
            $args[] = $registryDir;
        }
        $output[] = '$ dockan ' . implode(' ', $args);
        $output[] = command_text(run_dockan($dockan, $args));
    }
    $_GET['stack'] = $name;
    return trim(implode("\n", $output));
}

function backup_volume(string $dockan, string $name): string
{
    $safe = preg_replace('/[^A-Za-z0-9_.-]+/', '-', $name);
    $file = BACKUP_DIR . '/' . $safe . '-' . date('Ymd-His') . '.tar.gz';
    $out = run_dockan($dockan, ['volume', 'backup', $name, $file]);
    return trim(command_text($out) . "\n" . 'Backup: ' . $file);
}

function restore_volume(string $dockan): string
{
    $target = required_post('target');
    $backup = required_post('backup');
    $realBackup = realpath($backup);
    $realBackupDir = realpath(BACKUP_DIR);
    if ($realBackup === false || $realBackupDir === false || !str_starts_with($realBackup, $realBackupDir . DIRECTORY_SEPARATOR)) {
        throw new RuntimeException('Backup file is outside the Dockan UI backup directory.');
    }
    return command_text(run_dockan($dockan, ['volume', 'restore', $target, $realBackup]));
}

function required_post(string $key): string
{
    $value = trim((string) ($_POST[$key] ?? ''));
    if ($value === '') {
        throw new RuntimeException('Missing value: ' . $key);
    }
    return $value;
}

function run_dockan(string $dockan, array $args): array
{
    $cmd = array_merge([$dockan], $args);
    return run_command($cmd);
}

function run_command(array $cmd): array
{
    $command = implode(' ', array_map('escapeshellarg', $cmd));
    $home = getenv('HOME') ?: '';
    $oldPath = getenv('PATH') ?: '/usr/local/bin:/usr/bin:/bin';
    if ($home !== '') {
        putenv('PATH=' . $home . '/.local/bin:' . $oldPath);
    }
    $spec = [
        0 => ['pipe', 'r'],
        1 => ['pipe', 'w'],
        2 => ['pipe', 'w'],
    ];
    $process = proc_open($command, $spec, $pipes);
    putenv('PATH=' . $oldPath);
    if (!is_resource($process)) {
        throw new RuntimeException('Unable to run command.');
    }
    fclose($pipes[0]);
    $stdout = stream_get_contents($pipes[1]) ?: '';
    $stderr = stream_get_contents($pipes[2]) ?: '';
    fclose($pipes[1]);
    fclose($pipes[2]);
    $code = proc_close($process);
    return ['code' => $code, 'stdout' => $stdout, 'stderr' => $stderr, 'command' => $command];
}

function command_text(array $result): string
{
    $text = trim((string) $result['stdout'] . "\n" . (string) $result['stderr']);
    if ((int) $result['code'] !== 0) {
        throw new RuntimeException($text === '' ? 'Command failed.' : $text);
    }
    return $text;
}

function dashboard_content(string $dockan): string
{
    $containers = parse_table(command_or_empty($dockan, ['ps', '-a']));
    $images = parse_table(command_or_empty($dockan, ['images']));
    $volumes = parse_table(command_or_empty($dockan, ['volume', 'ls']));
    $networks = parse_table(command_or_empty($dockan, ['network', 'ls']));
    $doctor = command_or_empty($dockan, ['doctor']);
    return section('Overview', stats_grid([
        'Containers' => count($containers),
        'Images' => count($images),
        'Volumes' => count($volumes),
        'Networks' => count($networks),
    ])) .
    section('Quick Run', run_form($images)) .
    section('Doctor', '<pre>' . e($doctor) . '</pre>');
}

function containers_content(string $dockan): string
{
    $rows = parse_table(command_or_empty($dockan, ['ps', '-a']));
    $body = '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Status</th><th>PID</th><th>Image</th><th>Ports</th><th>Actions</th></tr></thead><tbody>';
    foreach ($rows as $row) {
        $name = $row['NAME'] ?? '';
        $status = strtolower($row['STATUS'] ?? '');
        $body .= '<tr>';
        $body .= '<td><a href="?view=logs&name=' . rawurlencode($name) . '">' . e($name) . '</a></td>';
        $body .= '<td>' . status_badge($status) . '</td>';
        $body .= '<td>' . e($row['PID'] ?? '') . '</td>';
        $body .= '<td>' . e($row['IMAGE'] ?? '') . '</td>';
        $body .= '<td>' . e($row['PORTS'] ?? '') . '</td>';
        $body .= '<td class="actions">' .
            post_button('health-container', ['name' => $name], 'Health') .
            post_button('stop-container', ['name' => $name], 'Stop') .
            post_button('remove-container', ['name' => $name], 'Remove', 'danger') .
            '</td>';
        $body .= '</tr>';
    }
    if (!$rows) {
        $body .= '<tr><td colspan="6" class="muted">No containers.</td></tr>';
    }
    $body .= '</tbody></table></div>';
    return section('Containers', $body) . section('Run Image', run_form(parse_table(command_or_empty($dockan, ['images']))));
}

function images_content(string $dockan): string
{
    $rows = parse_table(command_or_empty($dockan, ['images']));
    $body = '<div class="table-wrap"><table><thead><tr><th>Tag</th><th>Name</th><th>Path</th><th>Actions</th></tr></thead><tbody>';
    foreach ($rows as $row) {
        $tag = $row['TAG'] ?? '';
        $body .= '<tr><td>' . e($tag) . '</td><td>' . e($row['NAME'] ?? '') . '</td><td class="path">' . e($row['PATH'] ?? '') . '</td><td class="actions">' .
            post_button('remove-image', ['tag' => $tag], 'Remove', 'danger') .
            '</td></tr>';
    }
    if (!$rows) {
        $body .= '<tr><td colspan="4" class="muted">No images.</td></tr>';
    }
    $body .= '</tbody></table></div>';
    return section('Images', $body);
}

function volumes_content(string $dockan): string
{
    $rows = parse_table(command_or_empty($dockan, ['volume', 'ls']));
    $body = '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Path</th><th>Actions</th></tr></thead><tbody>';
    foreach ($rows as $row) {
        $name = $row['NAME'] ?? '';
        $body .= '<tr><td>' . e($name) . '</td><td class="path">' . e($row['PATH'] ?? '') . '</td><td class="actions">' .
            post_button('backup-volume', ['name' => $name], 'Backup') .
            post_button('remove-volume', ['name' => $name], 'Remove', 'danger') .
            '</td></tr>';
    }
    if (!$rows) {
        $body .= '<tr><td colspan="3" class="muted">No volumes.</td></tr>';
    }
    $body .= '</tbody></table></div>';
    $form = '<form method="post" class="inline-form">' . csrf_field() . '<input type="hidden" name="action" value="create-volume"><input name="name" placeholder="volume-name" required><button>Create Volume</button></form>';
    $backups = backups_list();
    return section('Volumes', $form . $body) . section('Backups', restore_form() . $backups);
}

function networks_content(string $dockan): string
{
    $rows = parse_table(command_or_empty($dockan, ['network', 'ls']));
    $body = '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Driver</th><th>Subnet</th><th>Bridge</th></tr></thead><tbody>';
    foreach ($rows as $row) {
        $body .= '<tr><td>' . e($row['NAME'] ?? '') . '</td><td>' . e($row['DRIVER'] ?? '') . '</td><td>' . e($row['SUBNET'] ?? '') . '</td><td>' . e($row['BRIDGE'] ?? '') . '</td></tr>';
    }
    if (!$rows) {
        $body .= '<tr><td colspan="4" class="muted">No networks.</td></tr>';
    }
    $body .= '</tbody></table></div>';
    return section('Networks', $body);
}

function compose_content(string $dockan): string
{
    $default = getcwd() . '/dockan.yml';
    $file = e((string) ($_POST['file'] ?? $_GET['file'] ?? $default));
    $body = '<form method="post" class="compose-form">' . csrf_field() .
        '<label>dockan.yml path<input name="file" value="' . $file . '" required></label>' .
        '<div class="actions">' .
        action_submit('compose-up', 'Up') .
        action_submit('compose-down', 'Down') .
        action_submit('compose-redeploy', 'Redeploy') .
        action_submit('compose-health', 'Health') .
        '</div></form>';
    return section('Compose', $body);
}

function stacks_content(string $dockan): string
{
    $stacks = stack_names();
    $selected = trim((string) ($_POST['stack'] ?? $_GET['stack'] ?? ''));
    if ($selected !== '') {
        try {
            $selected = clean_stack_name($selected);
        } catch (Throwable) {
            $selected = '';
        }
    } elseif ($stacks) {
        $selected = $stacks[0];
    }
    $yaml = $selected !== '' && is_file(stack_file($selected)) ? (string) file_get_contents(stack_file($selected)) : default_stack_yaml();
    $nameValue = $selected !== '' ? $selected : 'my-stack';
    $requiredImages = $selected !== '' && is_file(stack_required_images_file($selected))
        ? parse_image_list((string) file_get_contents(stack_required_images_file($selected)))
        : detect_stack_images($yaml);
    $registryDir = $selected !== '' && is_file(stack_registry_file($selected)) ? trim((string) file_get_contents(stack_registry_file($selected))) : '';

    $list = '<div class="table-wrap"><table><thead><tr><th>Name</th><th>File</th><th>Actions</th></tr></thead><tbody>';
    foreach ($stacks as $stack) {
        $list .= '<tr><td><a href="?view=stacks&stack=' . rawurlencode($stack) . '">' . e($stack) . '</a></td><td class="path">' . e(stack_file($stack)) . '</td><td class="actions">' .
            post_button('stack-up', ['stack' => $stack], 'Deploy') .
            post_button('stack-down', ['stack' => $stack], 'Stop') .
            post_button('stack-redeploy', ['stack' => $stack], 'Redeploy') .
            post_button('stack-health', ['stack' => $stack], 'Health') .
            post_button('stack-delete', ['stack' => $stack], 'Delete', 'danger') .
            '</td></tr>';
    }
    if (!$stacks) {
        $list .= '<tr><td colspan="3" class="muted">No stacks yet.</td></tr>';
    }
    $list .= '</tbody></table></div>';

    $editor = '<form method="post" class="stack-form">' . csrf_field() .
        ($selected !== '' ? '<input type="hidden" name="stack" value="' . e($selected) . '">' : '') .
        '<label>Stack name<input name="name" value="' . e($nameValue) . '" placeholder="my-stack" required></label>' .
        '<label>dockan.yml<textarea name="yaml" class="stack-editor" spellcheck="false" required>' . e($yaml) . '</textarea></label>' .
        '<label>Required images<textarea name="required_images" class="small-editor" spellcheck="false" placeholder="myapp:latest&#10;mariadb:local">' . e(implode("\n", $requiredImages)) . '</textarea></label>' .
        '<label>Registry folder<input name="registry_dir" value="' . e($registryDir) . '" placeholder="/home/anar/dockan-registry or empty for default"><span class="help">Local folder where Dockan looks for required images. Example: create it with <code>mkdir -p ~/dockan-registry</code>, then fill <code>/home/anar/dockan-registry</code>. The folder must already contain images pushed with <code>dockan push myapp:latest /home/anar/dockan-registry</code>.</span></label>' .
        '<div class="actions"><button name="action" value="stack-save">Save Stack</button>' .
        action_submit('stack-import-required', 'Import Required Images') .
        ($selected !== '' ? action_submit('stack-up', 'Deploy') . action_submit('stack-redeploy', 'Redeploy') . action_submit('stack-health', 'Health') : '') .
        '</div></form>';

    return section('Stacks', $list) . section($selected === '' ? 'Create Stack' : 'Edit Stack: ' . $selected, $editor);
}

function logs_content(string $dockan): string
{
    $name = trim((string) ($_GET['name'] ?? $_POST['name'] ?? ''));
    $logs = '';
    if ($name !== '') {
        $result = run_dockan($dockan, ['logs', $name]);
        $logs = trim((string) $result['stdout'] . "\n" . (string) $result['stderr']);
    }
    $form = '<form method="get" class="inline-form"><input type="hidden" name="view" value="logs"><input name="name" placeholder="container-name" value="' . e($name) . '" required><button>Show Logs</button></form>';
    return section('Logs', $form . '<pre>' . e($logs) . '</pre>');
}

function command_or_empty(string $dockan, array $args): string
{
    try {
        return command_text(run_dockan($dockan, $args));
    } catch (Throwable) {
        return '';
    }
}

function parse_table(string $text): array
{
    $lines = array_values(array_filter(array_map('rtrim', preg_split('/\R/', trim($text)) ?: [])));
    if (count($lines) < 2) {
        return [];
    }
    $headers = preg_split('/\s{2,}/', trim($lines[0])) ?: [];
    $rows = [];
    for ($i = 1; $i < count($lines); $i++) {
        $parts = preg_split('/\s{2,}/', trim($lines[$i]), count($headers)) ?: [];
        if (count($parts) === 1 && $parts[0] === '') {
            continue;
        }
        $row = [];
        foreach ($headers as $index => $header) {
            $row[$header] = $parts[$index] ?? '';
        }
        $rows[] = $row;
    }
    return $rows;
}

function run_form(array $images): string
{
    $options = '';
    foreach ($images as $image) {
        $tag = $image['TAG'] ?? '';
        if ($tag !== '') {
            $options .= '<option value="' . e($tag) . '">' . e($tag) . '</option>';
        }
    }
    if ($options === '') {
        $options = '<option value="">No local image</option>';
    }
    return '<form method="post" class="run-form">' . csrf_field() .
        '<input type="hidden" name="action" value="run-image">' .
        '<label>Name<input name="name" placeholder="myapp" required></label>' .
        '<label>Image<select name="image" required>' . $options . '</select></label>' .
        '<label>Port mapping<input name="ports" placeholder="8080:8080"></label>' .
        '<button>Run</button>' .
        '</form>';
}

function backups_list(): string
{
    $files = glob(BACKUP_DIR . '/*.tar.gz') ?: [];
    rsort($files);
    if (!$files) {
        return '<p class="muted">No backups.</p>';
    }
    $html = '<div class="table-wrap"><table><thead><tr><th>File</th><th>Size</th><th>Created</th></tr></thead><tbody>';
    foreach ($files as $file) {
        $html .= '<tr><td class="path">' . e($file) . '</td><td>' . e(human_bytes((int) filesize($file))) . '</td><td>' . e(date('Y-m-d H:i:s', (int) filemtime($file))) . '</td></tr>';
    }
    return $html . '</tbody></table></div>';
}

function restore_form(): string
{
    $files = glob(BACKUP_DIR . '/*.tar.gz') ?: [];
    rsort($files);
    $options = '';
    foreach ($files as $file) {
        $options .= '<option value="' . e($file) . '">' . e(basename($file)) . '</option>';
    }
    if ($options === '') {
        $options = '<option value="">No backup available</option>';
    }
    return '<form method="post" class="inline-form">' . csrf_field() .
        '<input type="hidden" name="action" value="restore-volume">' .
        '<select name="backup" required>' . $options . '</select>' .
        '<input name="target" placeholder="new-empty-volume" required>' .
        '<button>Restore Backup</button>' .
        '</form>';
}

function stats_grid(array $stats): string
{
    $html = '<div class="stats">';
    foreach ($stats as $label => $value) {
        $html .= '<div class="stat"><strong>' . e((string) $value) . '</strong><span>' . e($label) . '</span></div>';
    }
    return $html . '</div>';
}

function section(string $title, string $body): string
{
    return '<section><h2>' . e($title) . '</h2>' . $body . '</section>';
}

function post_button(string $action, array $fields, string $label, string $variant = ''): string
{
    $html = '<form method="post" class="button-form">' . csrf_field() . '<input type="hidden" name="action" value="' . e($action) . '">';
    foreach ($fields as $key => $value) {
        $html .= '<input type="hidden" name="' . e($key) . '" value="' . e((string) $value) . '">';
    }
    $class = $variant === '' ? '' : ' class="' . e($variant) . '"';
    return $html . '<button' . $class . '>' . e($label) . '</button></form>';
}

function action_submit(string $action, string $label): string
{
    return '<button name="action" value="' . e($action) . '">' . e($label) . '</button>';
}

function status_badge(string $status): string
{
    $class = str_contains($status, 'running') ? 'ok' : (str_contains($status, 'exit') || str_contains($status, 'stop') ? 'warn' : '');
    return '<span class="badge ' . e($class) . '">' . e($status === '' ? '-' : $status) . '</span>';
}

function csrf_field(): string
{
    return '<input type="hidden" name="csrf" value="' . e((string) ($_SESSION['csrf'] ?? '')) . '">';
}

function verify_csrf(): void
{
    $csrf = (string) ($_POST['csrf'] ?? '');
    if ($csrf === '' || !hash_equals((string) ($_SESSION['csrf'] ?? ''), $csrf)) {
        throw new RuntimeException('Invalid session token.');
    }
}

function locked_content(): string
{
    return '<main class="auth"><h1>Dockan Panel</h1><p>Set <code>DOCKAN_UI_TOKEN</code> before starting the PHP server.</p><pre>export DOCKAN_UI_TOKEN="change-me"
php -S 127.0.0.1:9090 index.php</pre></main>';
}

function login_content(?string $error): string
{
    return '<main class="auth"><h1>Dockan Panel</h1>' . ($error ? '<div class="alert danger">' . e($error) . '</div>' : '') .
        '<form method="post"><label>Access token<input type="password" name="token" autofocus required></label><button>Login</button></form></main>';
}

function render_page(string $title, string $content, bool $with_nav, ?string $flash = null, ?string $error = null): void
{
    $nav = $with_nav ? nav_html() : '';
    $messages = '';
    if ($flash) {
        $messages .= '<div class="alert">' . e($flash) . '</div>';
    }
    if ($error) {
        $messages .= '<div class="alert danger">' . e($error) . '</div>';
    }
    echo '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="icon" type="image/svg+xml" href="?asset=logo"><title>' . e($title) . ' - Dockan Panel</title><style>' . css() . '</style></head><body>' . $nav . '<main class="shell">' . $messages . $content . '</main></body></html>';
}

function nav_html(): string
{
    $items = [
        'dashboard' => 'Dashboard',
        'containers' => 'Containers',
        'images' => 'Images',
        'volumes' => 'Volumes',
        'networks' => 'Networks',
        'stacks' => 'Stacks',
        'compose' => 'Compose',
        'logs' => 'Logs',
    ];
    $html = '<header><div class="topbar"><a class="brand" href="?view=dashboard"><img src="?asset=logo" alt=""><span>Dockan Panel</span></a><nav>';
    foreach ($items as $key => $label) {
        $active = ($_GET['view'] ?? 'dashboard') === $key ? ' class="active"' : '';
        $html .= '<a' . $active . ' href="?view=' . e($key) . '">' . e($label) . '</a>';
    }
    $html .= '</nav><form method="post">' . csrf_field() . '<button name="logout" value="1">Logout</button></form></div></header>';
    return $html;
}

function page_title(string $view): string
{
    return ucwords(str_replace('-', ' ', $view));
}

function is_logged_in(): bool
{
    return ($_SESSION['dockan_ui_auth'] ?? false) === true;
}

function self_url(): string
{
    return strtok((string) ($_SERVER['REQUEST_URI'] ?? '/'), '?') ?: '/';
}

function ensure_storage(): void
{
    if (!is_dir(BACKUP_DIR)) {
        mkdir(BACKUP_DIR, 0755, true);
    }
    if (!is_dir(STACKS_DIR)) {
        mkdir(STACKS_DIR, 0755, true);
    }
}

function clean_stack_name(string $name): string
{
    $name = trim($name);
    if (!preg_match('/^[A-Za-z0-9_.-]{1,64}$/', $name)) {
        throw new RuntimeException('Invalid stack name. Use letters, numbers, dot, dash, or underscore.');
    }
    return $name;
}

function stack_dir(string $name): string
{
    return STACKS_DIR . '/' . clean_stack_name($name);
}

function stack_file(string $name): string
{
    return stack_dir($name) . '/dockan.yml';
}

function stack_required_images_file(string $name): string
{
    return stack_dir($name) . '/required-images.txt';
}

function stack_registry_file(string $name): string
{
    return stack_dir($name) . '/registry-dir.txt';
}

function persist_stack(string $name, string $yaml, array $requiredImages, string $registryDir): void
{
    if ($yaml === '') {
        throw new RuntimeException('Stack YAML is empty.');
    }
    if (strlen($yaml) > 512 * 1024) {
        throw new RuntimeException('Stack YAML is too large.');
    }
    $dir = stack_dir($name);
    if (!is_dir($dir) && !mkdir($dir, 0755, true)) {
        throw new RuntimeException('Unable to create stack directory.');
    }
    if (file_put_contents(stack_file($name), $yaml . "\n") === false) {
        throw new RuntimeException('Unable to save stack.');
    }
    if (!$requiredImages) {
        $requiredImages = detect_stack_images($yaml);
    }
    if (file_put_contents(stack_required_images_file($name), implode("\n", $requiredImages) . "\n") === false) {
        throw new RuntimeException('Unable to save required images.');
    }
    if (file_put_contents(stack_registry_file($name), $registryDir . "\n") === false) {
        throw new RuntimeException('Unable to save registry folder.');
    }
}

function stack_names(): array
{
    $entries = is_dir(STACKS_DIR) ? scandir(STACKS_DIR) : [];
    $names = [];
    foreach ($entries ?: [] as $entry) {
        if ($entry === '.' || $entry === '..') {
            continue;
        }
        if (is_dir(STACKS_DIR . '/' . $entry) && is_file(STACKS_DIR . '/' . $entry . '/dockan.yml')) {
            try {
                $names[] = clean_stack_name($entry);
            } catch (Throwable) {
            }
        }
    }
    sort($names);
    return $names;
}

function default_stack_yaml(): string
{
    return <<<'YAML'
name: my-stack
services:
  web:
    image: myapp:latest
    ports:
      - 8080:8080
    env:
      - PORT=8080
    restart: always
    healthcheck: CMD-SHELL curl -f http://127.0.0.1:8080/
YAML;
}

function detect_stack_images(string $yaml): array
{
    preg_match_all('/^\s*image:\s*["\']?([^"\'\s#]+)["\']?/m', $yaml, $matches);
    return parse_image_list(implode("\n", $matches[1] ?? []));
}

function parse_image_list(string $text): array
{
    $images = [];
    foreach (preg_split('/[\s,]+/', trim($text)) ?: [] as $image) {
        $image = trim($image);
        if ($image === '' || str_starts_with($image, '#')) {
            continue;
        }
        if (!preg_match('/^[A-Za-z0-9._\/:-]+$/', $image)) {
            throw new RuntimeException('Invalid image reference: ' . $image);
        }
        $images[$image] = true;
    }
    return array_keys($images);
}

function human_bytes(int $bytes): string
{
    $units = ['B', 'KB', 'MB', 'GB'];
    $value = (float) $bytes;
    foreach ($units as $unit) {
        if ($value < 1024 || $unit === 'GB') {
            return number_format($value, $unit === 'B' ? 0 : 1) . ' ' . $unit;
        }
        $value /= 1024;
    }
    return $bytes . ' B';
}

function e(string $value): string
{
    return htmlspecialchars($value, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8');
}

function css(): string
{
    return <<<'CSS'
:root {
  color-scheme: light;
  --bg: #f6f7f4;
  --panel: #ffffff;
  --ink: #17201b;
  --muted: #56635c;
  --line: #dfe5df;
  --accent: #176b48;
  --accent-dark: #0e4932;
  --accent-ink: #ffffff;
  --danger: #b42318;
  --ok: #067647;
  --warn: #a56110;
  --code: #101812;
  --code-line: #24342a;
  --shadow: 0 16px 40px rgba(23, 32, 27, 0.08);
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-size: 15px;
  line-height: 1.6;
}
header {
  border-bottom: 1px solid var(--line);
  background: var(--panel);
  position: sticky;
  top: 0;
  z-index: 3;
}
.topbar {
  min-height: 68px;
  width: min(1180px, calc(100vw - 48px));
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  font-weight: 800;
  color: var(--ink);
  text-decoration: none;
  white-space: nowrap;
  font-size: 1.08rem;
}
.brand img {
  width: 40px;
  height: 40px;
}
nav {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  flex: 1;
  font-size: 0.95rem;
}
nav a {
  color: var(--muted);
  text-decoration: none;
  padding: 8px 11px;
  border-radius: 8px;
  font-weight: 700;
}
nav a.active, nav a:hover {
  background: #eef6f1;
  color: var(--accent-dark);
}
.shell {
  width: min(1120px, calc(100vw - 48px));
  margin: 24px auto 48px;
}
section {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 18px;
  margin-bottom: 16px;
  box-shadow: 0 10px 28px rgba(23, 32, 27, 0.04);
}
h1, h2, h3 { margin: 0 0 14px; line-height: 1.2; }
h2 { font-size: 1.25rem; letter-spacing: 0; }
button, input, select {
  font: inherit;
}
button {
  border: 1px solid var(--accent);
  border-radius: 8px;
  background: var(--accent);
  color: var(--accent-ink);
  min-height: 38px;
  padding: 0 13px;
  cursor: pointer;
  white-space: nowrap;
  font-weight: 800;
}
button:hover { filter: brightness(0.96); }
button.danger, .danger button, .alert.danger { background: var(--danger); border-color: var(--danger); color: #fff; }
input, select, textarea {
  width: 100%;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fff;
  color: var(--ink);
}
input, select {
  min-height: 38px;
  padding: 0 10px;
}
textarea {
  min-height: 340px;
  padding: 12px;
  resize: vertical;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.5;
}
.small-editor {
  min-height: 100px;
}
input:focus, select:focus, textarea:focus {
  border-color: var(--accent);
  outline: 3px solid #eef6f1;
}
label { display: grid; gap: 6px; color: var(--muted); font-size: 13px; }
.help {
  display: block;
  max-width: 820px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.45;
}
.help code {
  color: var(--accent-dark);
}
pre {
  overflow: auto;
  padding: 14px;
  border-radius: 8px;
  background: var(--code);
  color: #eaf6ef;
  border: 1px solid var(--code-line);
  min-height: 54px;
}
.table-wrap { overflow-x: auto; }
table {
  width: 100%;
  border-collapse: collapse;
}
th, td {
  text-align: left;
  padding: 10px 8px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}
th {
  color: var(--muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0;
}
.path {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  max-width: 420px;
  overflow-wrap: anywhere;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.button-form { display: inline; }
.stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.stat {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 14px;
  background: #fbfcf9;
}
.stat strong { display: block; font-size: 28px; }
.stat span { color: var(--muted); }
.badge {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border-radius: 8px;
  background: #eef2f6;
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}
.badge.ok { color: var(--accent-dark); background: #eef6f1; }
.badge.warn { color: var(--warn); background: #fff4e4; }
.muted { color: var(--muted); }
.alert {
  border-radius: 8px;
  background: #eef6f1;
  color: var(--accent-dark);
  padding: 12px 14px;
  margin-bottom: 16px;
  white-space: pre-wrap;
  border: 1px solid var(--line);
}
.run-form, .compose-form, .inline-form {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  align-items: end;
  margin-bottom: 16px;
}
.compose-form { grid-template-columns: 1fr auto; }
.stack-form {
  display: grid;
  gap: 14px;
}
.auth {
  width: min(440px, calc(100vw - 32px));
  margin: 12vh auto;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 22px;
  box-shadow: var(--shadow);
}
.auth form { display: grid; gap: 14px; }
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
@media (max-width: 760px) {
  .topbar { align-items: flex-start; flex-direction: column; padding: 12px 0; width: min(100vw - 24px, 1180px); }
  header form { align-self: stretch; }
  header form button { width: 100%; }
  .stats, .run-form, .compose-form, .inline-form { grid-template-columns: 1fr; }
  .shell { width: min(100vw - 24px, 1120px); margin-top: 12px; }
}
CSS;
}
