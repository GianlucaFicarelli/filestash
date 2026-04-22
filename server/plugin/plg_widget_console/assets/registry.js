const commands = new Map();

export function register(cmd) {
    commands.set(cmd.id, cmd);
}

export function get(id) {
    return commands.get(id);
}

export function all() {
    return Array.from(commands.values());
}
