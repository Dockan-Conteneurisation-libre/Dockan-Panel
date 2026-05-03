<?php
declare(strict_types=1);

session_start();

const APP_NAME = 'Dockan Panel';
const STORAGE_DIR = __DIR__ . '/storage';
const BACKUP_DIR = STORAGE_DIR . '/backups';
const STACKS_DIR = STORAGE_DIR . '/stacks';
const TERMINALS_DIR = STORAGE_DIR . '/terminals';
const AUTH_FILE = STORAGE_DIR . '/auth-users.json';

ensure_storage();

if (($_GET['asset'] ?? '') === 'logo') {
    header('Content-Type: image/svg+xml');
    readfile(__DIR__ . '/dockan-logo.svg');
    exit;
}

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

if (!auth_has_users()) {
    if ($_SERVER['REQUEST_METHOD'] === 'POST' && ($_POST['setup'] ?? '') === '1') {
        try {
            create_first_admin();
            $_SESSION['csrf'] = bin2hex(random_bytes(24));
            header('Location: ' . self_url());
            exit;
        } catch (Throwable $e) {
            $error = $e->getMessage();
        }
    }
    render_page('Setup', setup_content($error), false);
    exit;
}

if (!is_logged_in()) {
    if (isset($_GET['webauthn'])) {
        handle_webauthn_api();
        exit;
    }
    if ($_SERVER['REQUEST_METHOD'] === 'POST' && ($_POST['login'] ?? '') === '1') {
        try {
            login_with_password();
        } catch (Throwable $e) {
            $error = $e->getMessage();
        }
    }
    render_page('Login', login_content($error), false);
    exit;
}

if (empty($_SESSION['csrf'])) {
    $_SESSION['csrf'] = bin2hex(random_bytes(24));
}

if (isset($_GET['terminal_api'])) {
    handle_terminal_api($dockan);
    exit;
}

if (isset($_GET['webauthn'])) {
    handle_webauthn_api();
    exit;
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
    'container' => container_content($dockan),
    'images' => images_content($dockan),
    'volumes' => volumes_content($dockan),
    'networks' => networks_content($dockan),
    'stacks' => stacks_content($dockan),
    'compose' => compose_content($dockan),
    'logs' => logs_content($dockan),
    'security' => security_content(),
    default => dashboard_content($dockan),
};

render_page(page_title($view), $content, true, $flash, $error);

function handle_action(string $action, string $dockan): string
{
    return match ($action) {
        'stop-container' => command_text(run_dockan($dockan, ['stop', required_post('name')])),
        'remove-container' => command_text(run_dockan($dockan, ['rm', required_post('name')])),
        'health-container' => command_text(run_dockan($dockan, ['health', required_post('name')])),
        'exec-container' => exec_container_command($dockan),
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
        'add-user' => add_user_action(),
        'delete-user' => delete_user_action(),
        'set-password' => set_password_action(),
        'begin-totp' => begin_totp_action(),
        'confirm-totp' => confirm_totp_action(),
        'disable-totp' => disable_totp_action(),
        'delete-passkey' => delete_passkey_action(),
        default => throw new RuntimeException('Unknown action.'),
    };
}

function run_image(string $dockan): string
{
    $name = required_post('name');
    $image = required_post('image');
    $ports = trim((string) ($_POST['ports'] ?? ''));
    $env = parse_multiline_values((string) ($_POST['env'] ?? ''));
    $volumes = parse_multiline_values((string) ($_POST['volumes'] ?? ''));
    $aliases = parse_multiline_values((string) ($_POST['aliases'] ?? ''));
    $network = trim((string) ($_POST['network'] ?? ''));
    $entrypoint = trim((string) ($_POST['entrypoint'] ?? ''));
    $restart = trim((string) ($_POST['restart'] ?? ''));
    $healthcheck = trim((string) ($_POST['healthcheck'] ?? ''));
    $memory = trim((string) ($_POST['memory'] ?? ''));
    $cpus = trim((string) ($_POST['cpus'] ?? ''));
    $isolation = trim((string) ($_POST['isolation'] ?? ''));
    $command = trim((string) ($_POST['command'] ?? ''));
    $args = ['run', '-d', '--name', $name];
    if ($ports !== '') {
        foreach (parse_multiline_values($ports) as $port) {
            $args[] = '-p';
            $args[] = $port;
        }
    }
    foreach ($env as $item) {
        $args[] = '-e';
        $args[] = $item;
    }
    foreach ($volumes as $item) {
        $args[] = '-v';
        $args[] = $item;
    }
    if ($network !== '') {
        $args[] = '--network';
        $args[] = $network;
    }
    foreach ($aliases as $item) {
        $args[] = '--alias';
        $args[] = $item;
    }
    if ($entrypoint !== '') {
        $args[] = '--entrypoint';
        $args[] = $entrypoint;
    }
    if ($restart !== '') {
        $args[] = '--restart';
        $args[] = $restart;
    }
    if ($healthcheck !== '') {
        $args[] = '--healthcheck';
        $args[] = $healthcheck;
    }
    if ($memory !== '') {
        $args[] = '--memory';
        $args[] = $memory;
    }
    if ($cpus !== '') {
        $args[] = '--cpus';
        $args[] = $cpus;
    }
    if (isset($_POST['gui'])) {
        $args[] = '--gui';
    }
    if ($isolation !== '') {
        $args[] = '--isolation=' . $isolation;
    }
    $args[] = $image;
    foreach (parse_command_values($command) as $item) {
        $args[] = $item;
    }
    return command_text(run_dockan($dockan, $args));
}

function exec_container_command(string $dockan): string
{
    $name = required_post('name');
    $command = trim((string) ($_POST['command'] ?? ''));
    if ($command === '') {
        throw new RuntimeException('Command is empty.');
    }
    if (strlen($command) > 8000) {
        throw new RuntimeException('Command is too large.');
    }
    return command_text(run_dockan($dockan, ['exec', $name, 'sh', '-lc', $command]));
}

function handle_terminal_api(string $dockan): void
{
    header('Content-Type: application/json; charset=utf-8');
    try {
        verify_csrf();
        $action = required_post('terminal_action');
        $payload = match ($action) {
            'start' => terminal_start($dockan, required_post('name')),
            'read' => terminal_read(required_post('id'), (int) ($_POST['offset'] ?? 0)),
            'input' => terminal_input(required_post('id'), (string) ($_POST['data'] ?? '')),
            'stop' => terminal_stop(required_post('id')),
            default => throw new RuntimeException('Unknown terminal action.'),
        };
        json_response(['ok' => true] + $payload);
    } catch (Throwable $e) {
        http_response_code(400);
        json_response(['ok' => false, 'error' => $e->getMessage()]);
    }
}

function terminal_start(string $dockan, string $name): array
{
    clean_resource_name($name, 'container name');
    $id = bin2hex(random_bytes(12));
    $dir = terminal_dir($id);
    if (!mkdir($dir, 0700, true)) {
        throw new RuntimeException('Unable to create terminal session.');
    }
    $output = $dir . '/output.log';
    $error = $dir . '/error.log';
    touch($output);
    touch($error);
    file_put_contents($dir . '/container', $name);

    $inner = shell_command(array_merge([$dockan], ['exec', $name, 'sh', '-li']));
    if (binary_exists('socat')) {
        return terminal_start_socat($id, $inner);
    }
    if (binary_exists('script')) {
        return terminal_start_script($id, $inner);
    }
    throw new RuntimeException('A live PTY terminal requires either "socat" or the util-linux "script" command.');
}

function terminal_start_script(string $id, string $inner): array
{
    $dir = terminal_dir_from_id($id);
    $input = $dir . '/input.fifo';
    $output = $dir . '/output.log';
    $error = $dir . '/error.log';
    if (!make_fifo($input)) {
        throw new RuntimeException('Unable to create terminal input pipe.');
    }
    $loop = 'while true; do cat ' . escapeshellarg($input) . '; sleep 0.05; done';
    $pty = 'TERM=xterm-256color script -qfec ' . escapeshellarg($inner) . ' ' . escapeshellarg($output);
    $background = close_extra_fds_shell() . '; setsid sh -c ' . escapeshellarg($loop . ' | ' . $pty) . ' >/dev/null 2>>' . escapeshellarg($error) . ' & echo $!';
    $result = run_command(['sh', '-lc', $background]);
    $pid = trim((string) $result['stdout']);
    if ((int) $result['code'] !== 0 || !preg_match('/^\d+$/', $pid)) {
        throw new RuntimeException(trim((string) $result['stderr']) ?: 'Unable to start terminal process.');
    }
    file_put_contents($dir . '/pid', $pid);
    usleep(180000);
    return terminal_read($id, 0) + ['id' => $id];
}

function terminal_start_socat(string $id, string $inner): array
{
    $dir = terminal_dir_from_id($id);
    $pty = $dir . '/terminal.pty';
    $output = $dir . '/output.log';
    $error = $dir . '/error.log';
    $ptyAddress = 'pty,raw,echo=0,link=' . $pty;
    $execAddress = 'exec:' . $inner . ',pty,setsid,ctty,stderr,sigint,sane';
    $socat = 'socat ' . escapeshellarg($ptyAddress) . ' ' . escapeshellarg($execAddress);
    $background = close_extra_fds_shell() . '; setsid sh -c ' . escapeshellarg($socat) . ' >/dev/null 2>>' . escapeshellarg($error) . ' & echo $!';
    $result = run_command(['sh', '-lc', $background]);
    $pid = trim((string) $result['stdout']);
    if ((int) $result['code'] !== 0 || !preg_match('/^\d+$/', $pid)) {
        throw new RuntimeException(trim((string) $result['stderr']) ?: 'Unable to start terminal process.');
    }
    file_put_contents($dir . '/pid', $pid);
    $deadline = microtime(true) + 2.0;
    while (!file_exists($pty) && microtime(true) < $deadline) {
        usleep(50000);
    }
    if (!file_exists($pty)) {
        throw new RuntimeException(trim((string) @file_get_contents($error)) ?: 'PTY device was not created.');
    }
    $reader = 'cat ' . escapeshellarg($pty) . ' >> ' . escapeshellarg($output);
    $readerBackground = close_extra_fds_shell() . '; setsid sh -c ' . escapeshellarg($reader) . ' >/dev/null 2>>' . escapeshellarg($error) . ' & echo $!';
    $readerResult = run_command(['sh', '-lc', $readerBackground]);
    $readerPid = trim((string) $readerResult['stdout']);
    if (preg_match('/^\d+$/', $readerPid)) {
        file_put_contents($dir . '/reader.pid', $readerPid);
    }
    usleep(180000);
    return terminal_read($id, 0) + ['id' => $id];
}

function terminal_read(string $id, int $offset): array
{
    $dir = terminal_dir_from_id($id);
    $output = $dir . '/output.log';
    $error = $dir . '/error.log';
    $size = is_file($output) ? filesize($output) : 0;
    if ($offset < 0 || $offset > $size) {
        $offset = 0;
    }
    $chunk = '';
    if ($size > $offset) {
        $chunk = file_get_contents($output, false, null, $offset, 65536) ?: '';
        $offset += strlen($chunk);
    }
    if ($chunk === '' && is_file($error) && filesize($error) > 0) {
        $chunk = file_get_contents($error) ?: '';
    }
    return ['output' => $chunk, 'offset' => $offset, 'alive' => terminal_alive($id)];
}

function terminal_input(string $id, string $data): array
{
    $dir = terminal_dir_from_id($id);
    if ($data === '') {
        return ['written' => 0];
    }
    if (strlen($data) > 4096) {
        throw new RuntimeException('Terminal input is too large.');
    }
    if (!terminal_alive($id)) {
        throw new RuntimeException('Terminal session is not running.');
    }
    $target = file_exists($dir . '/terminal.pty') ? $dir . '/terminal.pty' : $dir . '/input.fifo';
    $handle = @fopen($target, 'wb');
    if (!$handle) {
        throw new RuntimeException('Unable to open terminal input.');
    }
    $written = fwrite($handle, $data);
    fclose($handle);
    return ['written' => (int) $written];
}

function terminal_stop(string $id): array
{
    $dir = terminal_dir_from_id($id);
    $pid = terminal_pid($id);
    if ($pid > 0) {
        run_command(['sh', '-lc', 'kill -TERM -' . (int) $pid . ' 2>/dev/null || kill -TERM ' . (int) $pid . ' 2>/dev/null || true']);
    }
    $readerPid = trim((string) @file_get_contents($dir . '/reader.pid'));
    if (preg_match('/^\d+$/', $readerPid)) {
        run_command(['sh', '-lc', 'kill -TERM -' . (int) $readerPid . ' 2>/dev/null || kill -TERM ' . (int) $readerPid . ' 2>/dev/null || true']);
    }
    return ['stopped' => true, 'output' => "\n[terminal closed]\n", 'offset' => is_file($dir . '/output.log') ? filesize($dir . '/output.log') : 0, 'alive' => false];
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

function parse_multiline_values(string $text): array
{
    $values = [];
    foreach (preg_split('/\R+/', trim($text)) ?: [] as $line) {
        $line = trim($line);
        if ($line === '' || str_starts_with($line, '#')) {
            continue;
        }
        $values[] = $line;
    }
    return $values;
}

function parse_command_values(string $text): array
{
    if ($text === '') {
        return [];
    }
    $values = str_getcsv($text, ' ', '"', '\\');
    return array_values(array_filter(array_map('trim', $values), static fn (string $value): bool => $value !== ''));
}

function clean_resource_name(string $name, string $label): string
{
    if (!preg_match('/^[A-Za-z0-9_.-]{1,96}$/', $name)) {
        throw new RuntimeException('Invalid ' . $label . '.');
    }
    return $name;
}

function run_dockan(string $dockan, array $args): array
{
    $cmd = array_merge([$dockan], $args);
    return run_command($cmd);
}

function run_command(array $cmd): array
{
    $command = shell_command($cmd);
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

function shell_command(array $cmd): string
{
    return implode(' ', array_map('escapeshellarg', $cmd));
}

function close_extra_fds_shell(): string
{
    return 'if [ -d /proc/$$/fd ]; then for fd in /proc/$$/fd/*; do n=${fd##*/}; case "$n" in 0|1|2) ;; *) eval "exec $n>&-";; esac; done; fi';
}

function command_text(array $result): string
{
    $text = trim((string) $result['stdout'] . "\n" . (string) $result['stderr']);
    if ((int) $result['code'] !== 0) {
        throw new RuntimeException($text === '' ? 'Command failed.' : $text);
    }
    return $text;
}

function auth_has_users(): bool
{
    return count(auth_users()) > 0;
}

function auth_users(): array
{
    if (!is_file(AUTH_FILE)) {
        return [];
    }
    $data = json_decode((string) file_get_contents(AUTH_FILE), true);
    return is_array($data) && isset($data['users']) && is_array($data['users']) ? $data['users'] : [];
}

function save_auth_users(array $users): void
{
    $data = ['version' => 1, 'users' => array_values($users)];
    if (file_put_contents(AUTH_FILE, json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . "\n", LOCK_EX) === false) {
        throw new RuntimeException('Unable to save users.');
    }
    @chmod(AUTH_FILE, 0600);
}

function find_user(string $username): ?array
{
    foreach (auth_users() as $user) {
        if (hash_equals((string) ($user['username'] ?? ''), $username)) {
            return $user;
        }
    }
    return null;
}

function update_user(string $username, callable $callback): void
{
    $users = auth_users();
    foreach ($users as $index => $user) {
        if (($user['username'] ?? '') === $username) {
            $users[$index] = $callback($user);
            save_auth_users($users);
            return;
        }
    }
    throw new RuntimeException('User not found.');
}

function clean_username(string $username): string
{
    $username = strtolower(trim($username));
    if (!preg_match('/^[a-z0-9_.-]{2,32}$/', $username)) {
        throw new RuntimeException('Invalid username. Use 2-32 lowercase letters, numbers, dot, dash, or underscore.');
    }
    return $username;
}

function validate_password(string $password): void
{
    if (strlen($password) < 10) {
        throw new RuntimeException('Password must contain at least 10 characters.');
    }
}

function create_first_admin(): void
{
    if (auth_has_users()) {
        throw new RuntimeException('Setup already completed.');
    }
    $username = clean_username((string) ($_POST['username'] ?? 'admin'));
    $password = (string) ($_POST['password'] ?? '');
    $confirm = (string) ($_POST['confirm_password'] ?? '');
    validate_password($password);
    if (!hash_equals($password, $confirm)) {
        throw new RuntimeException('Passwords do not match.');
    }
    $user = [
        'username' => $username,
        'display_name' => $username,
        'role' => 'admin',
        'password_hash' => password_hash($password, PASSWORD_DEFAULT),
        'totp_secret' => '',
        'passkeys' => [],
        'created_at' => date(DATE_ATOM),
    ];
    save_auth_users([$user]);
}

function login_with_password(): void
{
    $username = clean_username((string) ($_POST['username'] ?? ''));
    $password = (string) ($_POST['password'] ?? '');
    $totp = trim((string) ($_POST['totp'] ?? ''));
    $user = find_user($username);
    if (!$user || !password_verify($password, (string) ($user['password_hash'] ?? ''))) {
        throw new RuntimeException('Invalid username or password.');
    }
    $secret = (string) ($user['totp_secret'] ?? '');
    if ($secret !== '') {
        if ($totp === '' || !totp_verify($secret, $totp)) {
            throw new RuntimeException('Invalid 2FA code.');
        }
    }
    complete_login($username);
}

function complete_login(string $username): void
{
    start_user_session($username);
    header('Location: ' . self_url());
    exit;
}

function start_user_session(string $username): void
{
    session_regenerate_id(true);
    $_SESSION['dockan_user'] = $username;
    $_SESSION['csrf'] = bin2hex(random_bytes(24));
}

function current_user(): ?array
{
    $username = (string) ($_SESSION['dockan_user'] ?? '');
    return $username === '' ? null : find_user($username);
}

function require_admin(): array
{
    $user = current_user();
    if (!$user || ($user['role'] ?? '') !== 'admin') {
        throw new RuntimeException('Admin rights required.');
    }
    return $user;
}

function add_user_action(): string
{
    require_admin();
    $username = clean_username(required_post('username'));
    $password = required_post('password');
    validate_password($password);
    if (find_user($username)) {
        throw new RuntimeException('User already exists.');
    }
    $users = auth_users();
    $users[] = [
        'username' => $username,
        'display_name' => $username,
        'role' => 'admin',
        'password_hash' => password_hash($password, PASSWORD_DEFAULT),
        'totp_secret' => '',
        'passkeys' => [],
        'created_at' => date(DATE_ATOM),
    ];
    save_auth_users($users);
    return 'User created: ' . $username;
}

function delete_user_action(): string
{
    $admin = require_admin();
    $username = clean_username(required_post('username'));
    if ($username === ($admin['username'] ?? '')) {
        throw new RuntimeException('You cannot delete your own account.');
    }
    $users = array_values(array_filter(auth_users(), static fn (array $user): bool => ($user['username'] ?? '') !== $username));
    if (count($users) === count(auth_users())) {
        throw new RuntimeException('User not found.');
    }
    if (!$users) {
        throw new RuntimeException('At least one admin user is required.');
    }
    save_auth_users($users);
    return 'User deleted: ' . $username;
}

function set_password_action(): string
{
    require_admin();
    $username = clean_username(required_post('username'));
    $password = required_post('password');
    validate_password($password);
    update_user($username, static function (array $user) use ($password): array {
        $user['password_hash'] = password_hash($password, PASSWORD_DEFAULT);
        return $user;
    });
    return 'Password updated: ' . $username;
}

function begin_totp_action(): string
{
    $user = require_admin();
    $secret = totp_secret();
    $_SESSION['pending_totp_secret'] = $secret;
    return "2FA secret generated. Scan it in your authenticator app, then enter the 6-digit code below.\nSecret: " . $secret;
}

function confirm_totp_action(): string
{
    $user = require_admin();
    $secret = (string) ($_SESSION['pending_totp_secret'] ?? '');
    if ($secret === '') {
        throw new RuntimeException('Generate a 2FA secret first.');
    }
    if (!totp_verify($secret, required_post('totp'))) {
        throw new RuntimeException('Invalid 2FA code.');
    }
    update_user((string) $user['username'], static function (array $item) use ($secret): array {
        $item['totp_secret'] = $secret;
        return $item;
    });
    unset($_SESSION['pending_totp_secret']);
    return '2FA enabled.';
}

function disable_totp_action(): string
{
    $user = require_admin();
    update_user((string) $user['username'], static function (array $item): array {
        $item['totp_secret'] = '';
        return $item;
    });
    unset($_SESSION['pending_totp_secret']);
    return '2FA disabled.';
}

function delete_passkey_action(): string
{
    $user = require_admin();
    $id = required_post('id');
    update_user((string) $user['username'], static function (array $item) use ($id): array {
        $item['passkeys'] = array_values(array_filter($item['passkeys'] ?? [], static fn (array $key): bool => ($key['id'] ?? '') !== $id));
        return $item;
    });
    return 'Passkey deleted.';
}

function totp_secret(): string
{
    $alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    $secret = '';
    for ($i = 0; $i < 32; $i++) {
        $secret .= $alphabet[random_int(0, strlen($alphabet) - 1)];
    }
    return $secret;
}

function totp_verify(string $secret, string $code): bool
{
    $code = preg_replace('/\D+/', '', $code) ?? '';
    if (strlen($code) !== 6) {
        return false;
    }
    $time = intdiv(time(), 30);
    for ($offset = -1; $offset <= 1; $offset++) {
        if (hash_equals(totp_code($secret, $time + $offset), $code)) {
            return true;
        }
    }
    return false;
}

function totp_code(string $secret, int $counter): string
{
    $key = base32_decode($secret);
    $binary = pack('N*', 0) . pack('N*', $counter);
    $hash = hash_hmac('sha1', $binary, $key, true);
    $offset = ord($hash[19]) & 0xf;
    $value = ((ord($hash[$offset]) & 0x7f) << 24) |
        ((ord($hash[$offset + 1]) & 0xff) << 16) |
        ((ord($hash[$offset + 2]) & 0xff) << 8) |
        (ord($hash[$offset + 3]) & 0xff);
    return str_pad((string) ($value % 1000000), 6, '0', STR_PAD_LEFT);
}

function base32_decode(string $text): string
{
    $alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    $text = strtoupper(preg_replace('/[^A-Z2-7]/', '', $text) ?? '');
    $bits = '';
    foreach (str_split($text) as $char) {
        $value = strpos($alphabet, $char);
        if ($value === false) {
            continue;
        }
        $bits .= str_pad(decbin($value), 5, '0', STR_PAD_LEFT);
    }
    $output = '';
    foreach (str_split($bits, 8) as $byte) {
        if (strlen($byte) === 8) {
            $output .= chr(bindec($byte));
        }
    }
    return $output;
}

function handle_webauthn_api(): void
{
    header('Content-Type: application/json; charset=utf-8');
    try {
        $action = (string) ($_GET['webauthn'] ?? '');
        $body = json_body();
        $payload = match ($action) {
            'register-options' => webauthn_register_options($body),
            'register-verify' => webauthn_register_verify($body),
            'login-options' => webauthn_login_options($body),
            'login-verify' => webauthn_login_verify($body),
            default => throw new RuntimeException('Unknown passkey action.'),
        };
        json_response(['ok' => true] + $payload);
    } catch (Throwable $e) {
        http_response_code(400);
        json_response(['ok' => false, 'error' => $e->getMessage()]);
    }
}

function json_body(): array
{
    $raw = (string) file_get_contents('php://input');
    if ($raw === '') {
        return [];
    }
    $data = json_decode($raw, true);
    if (!is_array($data)) {
        throw new RuntimeException('Invalid JSON payload.');
    }
    return $data;
}

function verify_json_csrf(array $body): void
{
    $csrf = (string) ($body['csrf'] ?? '');
    if ($csrf === '' || !hash_equals((string) ($_SESSION['csrf'] ?? ''), $csrf)) {
        throw new RuntimeException('Invalid session token.');
    }
}

function webauthn_register_options(array $body): array
{
    verify_json_csrf($body);
    $user = require_admin();
    $challenge = base64url_encode(random_bytes(32));
    $_SESSION['webauthn_register_challenge'] = $challenge;
    return [
        'publicKey' => [
            'challenge' => $challenge,
            'rp' => ['name' => APP_NAME],
            'user' => [
                'id' => base64url_encode((string) $user['username']),
                'name' => (string) $user['username'],
                'displayName' => (string) ($user['display_name'] ?? $user['username']),
            ],
            'pubKeyCredParams' => [
                ['type' => 'public-key', 'alg' => -7],
                ['type' => 'public-key', 'alg' => -257],
            ],
            'authenticatorSelection' => [
                'residentKey' => 'preferred',
                'userVerification' => 'preferred',
            ],
            'timeout' => 60000,
            'attestation' => 'none',
        ],
    ];
}

function webauthn_register_verify(array $body): array
{
    verify_json_csrf($body);
    $user = require_admin();
    $clientData = webauthn_client_data((string) ($body['clientDataJSON'] ?? ''), 'webauthn.create', (string) ($_SESSION['webauthn_register_challenge'] ?? ''));
    $id = (string) ($body['id'] ?? '');
    $publicKey = base64url_decode((string) ($body['publicKey'] ?? ''));
    if ($id === '' || $publicKey === '') {
        throw new RuntimeException('This browser did not return a usable passkey public key.');
    }
    $pem = der_public_key_to_pem($publicKey);
    if (!openssl_pkey_get_public($pem)) {
        throw new RuntimeException('Invalid passkey public key.');
    }
    update_user((string) $user['username'], static function (array $item) use ($id, $pem, $clientData): array {
        $keys = is_array($item['passkeys'] ?? null) ? $item['passkeys'] : [];
        foreach ($keys as $key) {
            if (($key['id'] ?? '') === $id) {
                throw new RuntimeException('This passkey is already registered.');
            }
        }
        $keys[] = [
            'id' => $id,
            'name' => 'Passkey ' . date('Y-m-d H:i'),
            'public_key' => $pem,
            'origin' => (string) ($clientData['origin'] ?? ''),
            'created_at' => date(DATE_ATOM),
        ];
        $item['passkeys'] = $keys;
        return $item;
    });
    unset($_SESSION['webauthn_register_challenge']);
    return ['message' => 'Passkey registered.'];
}

function webauthn_login_options(array $body): array
{
    $username = clean_username((string) ($body['username'] ?? ''));
    $user = find_user($username);
    $keys = is_array($user['passkeys'] ?? null) ? $user['passkeys'] : [];
    if (!$user || !$keys) {
        throw new RuntimeException('No passkey is registered for this user.');
    }
    $challenge = base64url_encode(random_bytes(32));
    $_SESSION['webauthn_login_challenge'] = $challenge;
    $_SESSION['webauthn_login_user'] = $username;
    return [
        'publicKey' => [
            'challenge' => $challenge,
            'allowCredentials' => array_map(static fn (array $key): array => [
                'type' => 'public-key',
                'id' => (string) ($key['id'] ?? ''),
            ], $keys),
            'userVerification' => 'preferred',
            'timeout' => 60000,
        ],
    ];
}

function webauthn_login_verify(array $body): array
{
    $username = clean_username((string) ($_SESSION['webauthn_login_user'] ?? ''));
    $user = find_user($username);
    $keys = is_array($user['passkeys'] ?? null) ? $user['passkeys'] : [];
    if (!$user || !$keys) {
        throw new RuntimeException('Passkey user not found.');
    }
    webauthn_client_data((string) ($body['clientDataJSON'] ?? ''), 'webauthn.get', (string) ($_SESSION['webauthn_login_challenge'] ?? ''));
    $id = (string) ($body['id'] ?? '');
    $match = null;
    foreach ($keys as $key) {
        if (($key['id'] ?? '') === $id) {
            $match = $key;
            break;
        }
    }
    if (!$match) {
        throw new RuntimeException('Unknown passkey.');
    }
    $authenticatorData = base64url_decode((string) ($body['authenticatorData'] ?? ''));
    $clientDataJSON = base64url_decode((string) ($body['clientDataJSON'] ?? ''));
    $signature = base64url_decode((string) ($body['signature'] ?? ''));
    if ($authenticatorData === '' || $clientDataJSON === '' || $signature === '') {
        throw new RuntimeException('Incomplete passkey response.');
    }
    $signed = $authenticatorData . hash('sha256', $clientDataJSON, true);
    $valid = openssl_verify($signed, $signature, (string) ($match['public_key'] ?? ''), OPENSSL_ALGO_SHA256);
    if ($valid !== 1) {
        throw new RuntimeException('Invalid passkey signature.');
    }
    unset($_SESSION['webauthn_login_challenge'], $_SESSION['webauthn_login_user']);
    start_user_session($username);
    return ['message' => 'Logged in.', 'redirect' => self_url()];
}

function webauthn_client_data(string $encoded, string $expectedType, string $expectedChallenge): array
{
    if ($expectedChallenge === '') {
        throw new RuntimeException('Missing passkey challenge.');
    }
    $json = base64url_decode($encoded);
    $data = json_decode($json, true);
    if (!is_array($data)) {
        throw new RuntimeException('Invalid passkey client data.');
    }
    if (($data['type'] ?? '') !== $expectedType) {
        throw new RuntimeException('Invalid passkey response type.');
    }
    if (!hash_equals($expectedChallenge, (string) ($data['challenge'] ?? ''))) {
        throw new RuntimeException('Invalid passkey challenge.');
    }
    webauthn_validate_origin((string) ($data['origin'] ?? ''));
    return $data;
}

function webauthn_validate_origin(string $origin): void
{
    $originHost = parse_url($origin, PHP_URL_HOST);
    $requestHost = explode(':', (string) ($_SERVER['HTTP_HOST'] ?? ''))[0] ?? '';
    if ($originHost === null || $originHost === false || $requestHost === '' || !hash_equals($requestHost, (string) $originHost)) {
        throw new RuntimeException('Invalid passkey origin.');
    }
}

function der_public_key_to_pem(string $der): string
{
    return "-----BEGIN PUBLIC KEY-----\n" . chunk_split(base64_encode($der), 64, "\n") . "-----END PUBLIC KEY-----\n";
}

function base64url_encode(string $value): string
{
    return rtrim(strtr(base64_encode($value), '+/', '-_'), '=');
}

function base64url_decode(string $value): string
{
    $value = strtr($value, '-_', '+/');
    $pad = strlen($value) % 4;
    if ($pad > 0) {
        $value .= str_repeat('=', 4 - $pad);
    }
    $decoded = base64_decode($value, true);
    if ($decoded === false) {
        throw new RuntimeException('Invalid base64url value.');
    }
    return $decoded;
}

function binary_exists(string $name): bool
{
    $result = run_command(['sh', '-lc', 'command -v ' . escapeshellarg($name) . ' >/dev/null 2>&1']);
    return (int) $result['code'] === 0;
}

function make_fifo(string $path): bool
{
    if (function_exists('posix_mkfifo')) {
        return @posix_mkfifo($path, 0600);
    }
    $result = run_command(['mkfifo', $path]);
    return (int) $result['code'] === 0;
}

function terminal_dir(string $id): string
{
    return TERMINALS_DIR . '/' . $id;
}

function terminal_dir_from_id(string $id): string
{
    if (!preg_match('/^[a-f0-9]{24}$/', $id)) {
        throw new RuntimeException('Invalid terminal session.');
    }
    $dir = terminal_dir($id);
    if (!is_dir($dir)) {
        throw new RuntimeException('Terminal session not found.');
    }
    return $dir;
}

function terminal_pid(string $id): int
{
    $dir = terminal_dir_from_id($id);
    $pid = trim((string) @file_get_contents($dir . '/pid'));
    return preg_match('/^\d+$/', $pid) ? (int) $pid : 0;
}

function terminal_alive(string $id): bool
{
    $pid = terminal_pid($id);
    if ($pid <= 0) {
        return false;
    }
    $result = run_command(['sh', '-lc', 'kill -0 ' . $pid . ' 2>/dev/null']);
    return (int) $result['code'] === 0;
}

function json_response(array $payload): void
{
    echo json_encode($payload, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
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
        $body .= '<td><a href="?view=container&name=' . rawurlencode($name) . '">' . e($name) . '</a></td>';
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
    return section('Containers', $body) . section('Create Container', run_form(parse_table(command_or_empty($dockan, ['images']))));
}

function container_content(string $dockan): string
{
    $name = trim((string) ($_GET['name'] ?? $_POST['name'] ?? ''));
    if ($name === '') {
        return section('Container', '<p class="muted">No container selected.</p>');
    }

    $containers = parse_table(command_or_empty($dockan, ['ps', '-a']));
    $current = null;
    foreach ($containers as $row) {
        if (($row['NAME'] ?? '') === $name) {
            $current = $row;
            break;
        }
    }
    if (!$current) {
        return section('Container', '<p class="muted">Container not found: ' . e($name) . '</p>');
    }

    $status = strtolower($current['STATUS'] ?? '');
    $summary = '<div class="container-head"><div><h2>' . e($name) . '</h2><p class="muted">' . e($current['IMAGE'] ?? '') . '</p></div>' . status_badge($status) . '</div>' .
        stats_grid([
            'PID' => $current['PID'] ?? '-',
            'Image' => $current['IMAGE'] ?? '-',
            'Ports' => $current['PORTS'] ?? '-',
            'Status' => $current['STATUS'] ?? '-',
        ]) .
        '<div class="actions detail-actions">' .
        post_button('health-container', ['name' => $name], 'Health') .
        post_button('stop-container', ['name' => $name], 'Stop') .
        post_button('remove-container', ['name' => $name], 'Remove', 'danger') .
        '<a class="button-link" href="?view=logs&name=' . rawurlencode($name) . '">Logs page</a>' .
        '<a class="button-link" href="?view=containers">Back</a>' .
        '</div>';

    $liveTerminal = '<div class="live-terminal-panel" data-container="' . e($name) . '" data-csrf="' . e((string) ($_SESSION['csrf'] ?? '')) . '">' .
        '<div class="actions terminal-toolbar">' .
        '<button type="button" data-terminal-start>Connect</button>' .
        '<button type="button" data-terminal-stop class="danger">Disconnect</button>' .
        '<button type="button" data-terminal-clear>Clear</button>' .
        '<span class="terminal-state" data-terminal-state>disconnected</span>' .
        '</div>' .
        '<pre class="live-terminal" data-terminal-output tabindex="0"></pre>' .
        '<p class="help">Click inside the terminal, then type. Enter, Backspace, arrows, Tab, Ctrl+C, Ctrl+D and paste are sent to the running shell.</p>' .
        '</div>';

    $command = trim((string) ($_POST['command'] ?? 'pwd && ls -la'));
    $quickExec = '<form method="post" class="terminal-form">' . csrf_field() .
        '<input type="hidden" name="action" value="exec-container">' .
        '<input type="hidden" name="name" value="' . e($name) . '">' .
        '<label>Quick command<textarea name="command" class="small-editor" spellcheck="false" required>' . e($command) . '</textarea><span class="help">One-shot fallback through <code>dockan exec ' . e($name) . ' sh -lc "..."</code>.</span></label>' .
        '<button>Run Command</button>' .
        '</form>';

    $inspect = command_or_empty($dockan, ['inspect', $name]);
    $logs = command_or_empty($dockan, ['logs', $name]);

    return section('Container', $summary) .
        section('Live Terminal', $liveTerminal) .
        section('Quick Exec', $quickExec) .
        section('Inspect', '<pre>' . e($inspect) . '</pre>') .
        section('Logs', '<pre>' . e($logs) . '</pre>');
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

function security_content(): string
{
    $user = require_admin();
    $users = auth_users();
    $rows = '<div class="table-wrap"><table><thead><tr><th>User</th><th>Role</th><th>2FA</th><th>Passkeys</th><th>Actions</th></tr></thead><tbody>';
    foreach ($users as $item) {
        $username = (string) ($item['username'] ?? '');
        $passkeys = is_array($item['passkeys'] ?? null) ? count($item['passkeys']) : 0;
        $rows .= '<tr><td>' . e($username) . '</td><td>' . e((string) ($item['role'] ?? 'admin')) . '</td><td>' . (($item['totp_secret'] ?? '') !== '' ? status_badge('enabled') : status_badge('disabled')) . '</td><td>' . e((string) $passkeys) . '</td><td class="actions">' .
            post_button('delete-user', ['username' => $username], 'Delete', 'danger') .
            '</td></tr>';
    }
    $rows .= '</tbody></table></div>';

    $add = '<form method="post" class="inline-form">' . csrf_field() .
        '<input type="hidden" name="action" value="add-user">' .
        '<input name="username" placeholder="admin2" required>' .
        '<input type="password" name="password" placeholder="temporary-password" required>' .
        '<button>Add Admin</button></form>';

    $password = '<form method="post" class="inline-form">' . csrf_field() .
        '<input type="hidden" name="action" value="set-password">' .
        '<input name="username" value="' . e((string) $user['username']) . '" required>' .
        '<input type="password" name="password" placeholder="new-password" required>' .
        '<button>Set Password</button></form>';

    $pending = (string) ($_SESSION['pending_totp_secret'] ?? '');
    $totpUri = $pending !== '' ? totp_uri((string) $user['username'], $pending) : '';
    $totp = '<div class="security-grid">' .
        '<div>' . (($user['totp_secret'] ?? '') !== '' ? '<p class="muted">2FA is enabled for your account.</p>' : '<p class="muted">2FA is disabled for your account.</p>') .
        '<div class="actions">' .
        post_button('begin-totp', [], 'Generate 2FA Secret') .
        (($user['totp_secret'] ?? '') !== '' ? post_button('disable-totp', [], 'Disable 2FA', 'danger') : '') .
        '</div></div>' .
        ($pending !== '' ? '<div><label>Authenticator URI<input readonly value="' . e($totpUri) . '"></label><form method="post" class="inline-form">' . csrf_field() . '<input type="hidden" name="action" value="confirm-totp"><input name="totp" placeholder="123456" required><button>Enable 2FA</button></form></div>' : '') .
        '</div>';

    $passkeys = '<div class="passkey-panel" data-passkey-user="' . e((string) $user['username']) . '" data-csrf="' . e((string) ($_SESSION['csrf'] ?? '')) . '">' .
        '<div class="actions"><button type="button" data-passkey-register>Add Passkey</button><span class="muted" data-passkey-status></span></div>' .
        '<p class="muted">Passkeys work on localhost or HTTPS and are stored only in this panel user database.</p>' .
        passkeys_list($user) .
        '</div>';

    return section('Users', $add . $rows) .
        section('Password', $password) .
        section('Two-Factor Authentication', $totp) .
        section('Passkeys', $passkeys);
}

function passkeys_list(array $user): string
{
    $keys = is_array($user['passkeys'] ?? null) ? $user['passkeys'] : [];
    if (!$keys) {
        return '<p class="muted">No passkeys registered.</p>';
    }
    $html = '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Created</th><th>Actions</th></tr></thead><tbody>';
    foreach ($keys as $key) {
        $html .= '<tr><td>' . e((string) ($key['name'] ?? 'Passkey')) . '</td><td>' . e((string) ($key['created_at'] ?? '')) . '</td><td>' .
            post_button('delete-passkey', ['id' => (string) ($key['id'] ?? '')], 'Delete', 'danger') .
            '</td></tr>';
    }
    return $html . '</tbody></table></div>';
}

function totp_uri(string $username, string $secret): string
{
    return 'otpauth://totp/' . rawurlencode(APP_NAME . ':' . $username) . '?secret=' . rawurlencode($secret) . '&issuer=' . rawurlencode(APP_NAME) . '&algorithm=SHA1&digits=6&period=30';
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
    $basic = '<div class="run-basic">' .
        '<label>Name<input name="name" placeholder="myapp" required></label>' .
        '<label>Image<select name="image" required>' . $options . '</select></label>' .
        '<label>Ports<input name="ports" placeholder="8080:8080"></label>' .
        '<button>Create</button>' .
        '</div>';
    $advanced = '<details class="advanced-options"><summary>Advanced options</summary><div class="advanced-grid">' .
        '<label>Volumes<textarea name="volumes" class="mini-editor" spellcheck="false" placeholder="app-data:/app/data&#10;/home/anar/site:/app/site:ro"></textarea><span class="help">One mount per line.</span></label>' .
        '<label>Environment<textarea name="env" class="mini-editor" spellcheck="false" placeholder="PORT=8080&#10;APP_ENV=prod"></textarea><span class="help">One KEY=VALUE per line.</span></label>' .
        '<label>Network<input name="network" placeholder="host or my-network"></label>' .
        '<label>Aliases<textarea name="aliases" class="mini-editor" spellcheck="false" placeholder="web&#10;api"></textarea></label>' .
        '<label>Entrypoint<input name="entrypoint" placeholder="/bin/sh"></label>' .
        '<label>Command<input name="command" placeholder="-lc &quot;php -S 0.0.0.0:8080&quot;"></label>' .
        '<label>Restart<select name="restart"><option value="">Image default</option><option value="no">no</option><option value="always">always</option><option value="on-failure">on-failure</option></select></label>' .
        '<label>Healthcheck<input name="healthcheck" placeholder="CMD-SHELL curl -f http://127.0.0.1:8080/"></label>' .
        '<label>Memory<input name="memory" placeholder="512m"></label>' .
        '<label>CPUs<input name="cpus" placeholder="1.5"></label>' .
        '<label>Isolation<select name="isolation"><option value="">auto</option><option value="none">none</option><option value="firejail">firejail</option><option value="bubblewrap">bubblewrap</option><option value="systemd-nspawn">systemd-nspawn</option><option value="chroot">chroot</option></select></label>' .
        '<label class="check-row"><input type="checkbox" name="gui" value="1"> GUI sockets</label>' .
        '</div></details>';
    return '<form method="post" class="run-form">' . csrf_field() .
        '<input type="hidden" name="action" value="run-image">' .
        $basic . $advanced .
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

function login_content(?string $error): string
{
    return '<main class="auth"><h1>Dockan Panel</h1>' . ($error ? '<div class="alert danger">' . e($error) . '</div>' : '') .
        '<form method="post"><input type="hidden" name="login" value="1"><label>Username<input name="username" value="admin" autofocus required></label><label>Password<input type="password" name="password" required></label><label>2FA code<input name="totp" inputmode="numeric" placeholder="optional"></label><button>Login</button></form>' .
        '<div class="passkey-panel login-passkey"><button type="button" data-passkey-login>Login with Passkey</button><span class="muted" data-passkey-status></span></div></main>';
}

function setup_content(?string $error): string
{
    return '<main class="auth"><h1>Setup Admin</h1>' . ($error ? '<div class="alert danger">' . e($error) . '</div>' : '') .
        '<form method="post"><input type="hidden" name="setup" value="1"><label>Username<input name="username" value="admin" autofocus required></label><label>Password<input type="password" name="password" required></label><label>Confirm password<input type="password" name="confirm_password" required></label><button>Create Admin</button></form></main>';
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
    echo '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="icon" type="image/svg+xml" href="?asset=logo"><title>' . e($title) . ' - Dockan Panel</title><style>' . css() . '</style></head><body>' . $nav . '<main class="shell">' . $messages . $content . '</main><script>' . terminal_js() . '</script></body></html>';
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
        'security' => 'Security',
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
    return current_user() !== null;
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
    if (!is_dir(TERMINALS_DIR)) {
        mkdir(TERMINALS_DIR, 0700, true);
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

function terminal_js(): string
{
    return <<<'JS'
(() => {
  const ansi = /\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1B\\))/g;
  const clean = (text) => text
    .replace(ansi, '')
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .split('\n')
    .filter((line) => !line.includes('nsenter: reassociate to namespaces failed') && !line.includes('[dockan] nsenter impossible'))
    .join('\n');
  const api = async (panel, terminalAction, extra = {}) => {
    const body = new FormData();
    body.set('csrf', panel.dataset.csrf || '');
    body.set('terminal_action', terminalAction);
    for (const [key, value] of Object.entries(extra)) {
      body.set(key, value);
    }
    const response = await fetch('?terminal_api=1', { method: 'POST', body });
    const data = await response.json();
    if (!data.ok) throw new Error(data.error || 'Terminal error.');
    return data;
  };
  document.querySelectorAll('[data-container][data-csrf]').forEach((panel) => {
    const output = panel.querySelector('[data-terminal-output]');
    const state = panel.querySelector('[data-terminal-state]');
    const start = panel.querySelector('[data-terminal-start]');
    const stop = panel.querySelector('[data-terminal-stop]');
    const clear = panel.querySelector('[data-terminal-clear]');
    let id = '';
    let offset = 0;
    let timer = 0;
    let buffer = '';
    let errors = 0;
    const setState = (text) => { state.textContent = text; };
    const append = (text) => {
      if (!text) return;
      buffer += clean(text);
      if (buffer.length > 80000) buffer = buffer.slice(-80000);
      output.textContent = buffer;
      output.scrollTop = output.scrollHeight;
    };
    const poll = async () => {
      if (!id) return;
      try {
        const data = await api(panel, 'read', { id, offset: String(offset) });
        errors = 0;
        offset = data.offset || offset;
        append(data.output || '');
        setState(data.alive ? 'connected' : 'closed');
        if (!data.alive) {
          clearInterval(timer);
          timer = 0;
        }
      } catch (error) {
        errors += 1;
        if (errors >= 2) {
          append('\n[terminal] connection lost. Refresh the page or reconnect.\n');
        }
        setState('error');
        clearInterval(timer);
        timer = 0;
        id = '';
      }
    };
    const send = async (data) => {
      if (!id) return;
      try {
        await api(panel, 'input', { id, data });
        setTimeout(poll, 50);
      } catch (error) {
        append('\n[terminal] connection lost. Refresh the page or reconnect.\n');
        setState('error');
        clearInterval(timer);
        timer = 0;
        id = '';
      }
    };
    start.addEventListener('click', async () => {
      if (id) return output.focus();
      setState('connecting');
      buffer = '';
      output.textContent = '';
      try {
        const data = await api(panel, 'start', { name: panel.dataset.container });
        id = data.id;
        offset = data.offset || 0;
        append(data.output || '');
        setState(data.alive ? 'connected' : 'closed');
        timer = window.setInterval(poll, 700);
        output.focus();
      } catch (error) {
        append('[terminal] ' + error.message + '\n');
        setState('error');
      }
    });
    stop.addEventListener('click', async () => {
      if (!id) return;
      try {
        const data = await api(panel, 'stop', { id });
        append(data.output || '\n[terminal closed]\n');
      } catch (error) {
        append('\n[terminal] ' + error.message + '\n');
      }
      id = '';
      offset = 0;
      clearInterval(timer);
      timer = 0;
      setState('disconnected');
    });
    clear.addEventListener('click', () => {
      buffer = '';
      output.textContent = '';
      output.focus();
    });
    output.addEventListener('click', () => output.focus());
    output.addEventListener('paste', (event) => {
      const text = event.clipboardData ? event.clipboardData.getData('text') : '';
      if (text) {
        event.preventDefault();
        send(text);
      }
    });
    output.addEventListener('keydown', (event) => {
      if (!id) {
        if (event.key === 'Backspace') {
          event.preventDefault();
        }
        return;
      }
      let data = '';
      if (event.ctrlKey && event.key.toLowerCase() === 'c') data = '\x03';
      else if (event.ctrlKey && event.key.toLowerCase() === 'd') data = '\x04';
      else if (event.ctrlKey && event.key.toLowerCase() === 'l') data = '\x0c';
      else if (event.key === 'Enter') data = '\n';
      else if (event.key === 'Backspace') data = '\x7f';
      else if (event.key === 'Tab') data = '\t';
      else if (event.key === 'ArrowUp') data = '\x1b[A';
      else if (event.key === 'ArrowDown') data = '\x1b[B';
      else if (event.key === 'ArrowRight') data = '\x1b[C';
      else if (event.key === 'ArrowLeft') data = '\x1b[D';
      else if (!event.ctrlKey && !event.metaKey && event.key.length === 1) data = event.key;
      if (data !== '') {
        event.preventDefault();
        send(data);
      }
    });
  });
  const b64ToBytes = (value) => {
    value = value.replace(/-/g, '+').replace(/_/g, '/');
    value += '='.repeat((4 - (value.length % 4)) % 4);
    return Uint8Array.from(atob(value), (char) => char.charCodeAt(0));
  };
  const bytesToB64 = (value) => {
    const bytes = value instanceof Uint8Array ? value : new Uint8Array(value);
    let binary = '';
    for (const byte of bytes) binary += String.fromCharCode(byte);
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  };
  const passkeyPost = async (action, payload = {}) => {
    const response = await fetch('?webauthn=' + encodeURIComponent(action), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await response.json();
    if (!data.ok) throw new Error(data.error || 'Passkey error.');
    return data;
  };
  const prepareCreateOptions = (options) => {
    options.challenge = b64ToBytes(options.challenge);
    options.user.id = b64ToBytes(options.user.id);
    if (options.excludeCredentials) {
      options.excludeCredentials = options.excludeCredentials.map((item) => ({ ...item, id: b64ToBytes(item.id) }));
    }
    return options;
  };
  const prepareGetOptions = (options) => {
    options.challenge = b64ToBytes(options.challenge);
    if (options.allowCredentials) {
      options.allowCredentials = options.allowCredentials.map((item) => ({ ...item, id: b64ToBytes(item.id) }));
    }
    return options;
  };
  const setPasskeyStatus = (root, text) => {
    const status = root.querySelector('[data-passkey-status]');
    if (status) status.textContent = text;
  };
  document.querySelectorAll('[data-passkey-register]').forEach((button) => {
    button.addEventListener('click', async () => {
      const panel = button.closest('[data-passkey-user]');
      try {
        if (!window.PublicKeyCredential) throw new Error('Passkeys are not supported by this browser.');
        setPasskeyStatus(panel, 'waiting for browser');
        const options = await passkeyPost('register-options', { csrf: panel.dataset.csrf || '' });
        const credential = await navigator.credentials.create({ publicKey: prepareCreateOptions(options.publicKey) });
        if (!credential.response.getPublicKey) {
          throw new Error('This browser cannot export the public key needed by Dockan Panel.');
        }
        await passkeyPost('register-verify', {
          csrf: panel.dataset.csrf || '',
          id: credential.id,
          rawId: bytesToB64(credential.rawId),
          clientDataJSON: bytesToB64(credential.response.clientDataJSON),
          publicKey: bytesToB64(credential.response.getPublicKey())
        });
        setPasskeyStatus(panel, 'registered');
        window.location.reload();
      } catch (error) {
        setPasskeyStatus(panel, error.message);
      }
    });
  });
  document.querySelectorAll('[data-passkey-login]').forEach((button) => {
    button.addEventListener('click', async () => {
      const panel = button.closest('.login-passkey');
      const username = document.querySelector('input[name="username"]')?.value || '';
      try {
        if (!window.PublicKeyCredential) throw new Error('Passkeys are not supported by this browser.');
        setPasskeyStatus(panel, 'waiting for browser');
        const options = await passkeyPost('login-options', { username });
        const assertion = await navigator.credentials.get({ publicKey: prepareGetOptions(options.publicKey) });
        const result = await passkeyPost('login-verify', {
          id: assertion.id,
          rawId: bytesToB64(assertion.rawId),
          clientDataJSON: bytesToB64(assertion.response.clientDataJSON),
          authenticatorData: bytesToB64(assertion.response.authenticatorData),
          signature: bytesToB64(assertion.response.signature),
          userHandle: assertion.response.userHandle ? bytesToB64(assertion.response.userHandle) : ''
        });
        window.location.href = result.redirect || '/';
      } catch (error) {
        setPasskeyStatus(panel, error.message);
      }
    });
  });
})();
JS;
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
.mini-editor {
  min-height: 78px;
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
.detail-actions {
  margin-top: 14px;
}
.container-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}
.container-head h2 {
  margin-bottom: 4px;
}
.container-head p {
  margin: 0;
}
.button-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 38px;
  padding: 0 13px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fff;
  color: var(--accent-dark);
  font-weight: 800;
  text-decoration: none;
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
.compose-form, .inline-form {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  align-items: end;
  margin-bottom: 16px;
}
.run-form {
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
}
.run-basic {
  display: grid;
  grid-template-columns: minmax(130px, 1fr) minmax(180px, 1.3fr) minmax(140px, 1fr) auto;
  gap: 12px;
  align-items: end;
}
.advanced-options {
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fbfcf9;
}
.advanced-options summary {
  min-height: 38px;
  display: flex;
  align-items: center;
  padding: 0 12px;
  color: var(--accent-dark);
  cursor: pointer;
  font-weight: 800;
}
.advanced-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  padding: 12px;
  border-top: 1px solid var(--line);
}
.compose-form { grid-template-columns: 1fr auto; }
.stack-form {
  display: grid;
  gap: 14px;
}
.security-grid {
  display: grid;
  grid-template-columns: 1fr 1.4fr;
  gap: 14px;
  align-items: start;
}
.passkey-panel {
  display: grid;
  gap: 12px;
}
.login-passkey {
  margin-top: 14px;
}
.terminal-form {
  display: grid;
  gap: 12px;
}
.terminal-toolbar {
  align-items: center;
  margin-bottom: 10px;
}
.terminal-state {
  min-height: 28px;
  display: inline-flex;
  align-items: center;
  color: var(--muted);
  font-size: 12px;
  font-weight: 800;
  text-transform: uppercase;
}
.live-terminal {
  min-height: 420px;
  max-height: 62vh;
  margin: 0;
  white-space: pre-wrap;
  overflow: auto;
  caret-color: #eaf6ef;
}
.live-terminal:focus {
  outline: 3px solid #bfe8d3;
}
.check-row {
  align-self: end;
  min-height: 38px;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--ink);
  font-weight: 800;
}
.check-row input {
  width: 18px;
  height: 18px;
  min-height: 18px;
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
  .stats, .compose-form, .inline-form, .run-basic, .advanced-grid, .security-grid { grid-template-columns: 1fr; }
  .shell { width: min(100vw - 24px, 1120px); margin-top: 12px; }
}
CSS;
}
