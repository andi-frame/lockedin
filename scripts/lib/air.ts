/**
 * The air command for one Go process. With AIR_POLL=1 it polls for changes: file-system events do
 * not cross Docker bind mounts on Windows and macOS (docs/RUNNING.md §4). air reads the .toml
 * itself and has no env expansion, so the switch is passed as flags.
 */
export function airCommand(configFile: string, env: Record<string, string | undefined>): string[] {
  const cmd = ["air", "-c", configFile];
  if (env.AIR_POLL === "1") cmd.push("--build.poll", "true", "--build.poll_interval", "500");
  return cmd;
}
