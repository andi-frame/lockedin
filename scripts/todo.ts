// Placeholder for root scripts whose implementation belongs to a later PLAN task.
// Exits non-zero so CI never mistakes a stub for a passing step.
const [cmd = "this command", task = "?"] = process.argv.slice(2);
console.error(`${cmd} is not implemented yet (docs/PLAN.md task ${task}).`);
process.exit(2);
