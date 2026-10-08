// `tepatictl seed` prints one `backer <email>  password <pw>` and one `doer ...` line.
export function parseSeedLogin(stdout: string, role: "backer" | "doer"): { email: string; password: string } | undefined {
  const m = new RegExp(`^${role}\\s+(\\S+)\\s+password\\s+(\\S+)`, "m").exec(stdout);
  return m?.[1] && m[2] ? { email: m[1], password: m[2] } : undefined;
}
