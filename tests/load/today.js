// k6 load test (PLAN 8.4): mixed reads and proof edits against a deploy, as the doers of seeded
// `today` scenarios. Run it with `bun run load -- --env staging`, which seeds the users, writes
// .users.json next to this file and starts k6 from its image on the deploy's network.
//
// Reads (READ_RATE a second): Today 40 %, pacts list 15 %, a pact's ledger 20 %, notifications 15 %,
// review queue 10 %. Writes (WRITE_RATE a second): the doer sends or edits today's proof. A check-in
// takes edits until its deadline (each is a new proof version), which is what makes a steady write
// rate possible with a few users. The thresholds are SPEC section 10: p95 150 ms for reads, 300 ms
// for writes, nearly no errors.
import http from "k6/http";
import { check, fail } from "k6";
import exec from "k6/execution";
import { SharedArray } from "k6/data";

const users = new SharedArray("users", () => JSON.parse(open("/load/.users.json")));
const API = "https://localhost/api/v1";
const DURATION = __ENV.DURATION || "5m";
const READ_RATE = Number(__ENV.READ_RATE || 200);
const WRITE_RATE = Number(__ENV.WRITE_RATE || 20);

export const options = {
  // Caddy has a certificate and a site only for "localhost": keep the name, send the packets to its container.
  hosts: { localhost: __ENV.CADDY_IP },
  insecureSkipTLSVerify: true,
  scenarios: {
    reads: {
      executor: "constant-arrival-rate",
      rate: READ_RATE,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(20, Math.ceil(READ_RATE / 2)),
      maxVUs: Math.max(100, READ_RATE * 2),
      exec: "read",
    },
    writes: {
      executor: "constant-arrival-rate",
      rate: WRITE_RATE,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: Math.max(5, Math.ceil(WRITE_RATE / 2)),
      maxVUs: Math.max(50, WRITE_RATE * 4),
      exec: "write",
    },
  },
  thresholds: {
    "http_req_duration{kind:read}": ["p(95)<150"],
    "http_req_duration{kind:write}": ["p(95)<300"],
    "http_req_failed{kind:read}": ["rate<0.01"],
    "http_req_failed{kind:write}": ["rate<0.01"],
    checks: ["rate>0.99"],
  },
  summaryTrendStats: ["avg", "med", "p(90)", "p(95)", "p(99)", "max"],
};

const hex = (n) => Array.from({ length: n }, () => Math.floor(Math.random() * 16).toString(16)).join("");
const uuid = () => `${hex(8)}-${hex(4)}-4${hex(3)}-a${hex(3)}-${hex(12)}`;

// One session per seeded user. Cookies are sent by hand: k6's own jar would mix the users up.
export function setup() {
  const sessions = [];
  for (const u of users) {
    const login = http.post(`${API}/auth/login`, JSON.stringify({ email: u.email, password: u.password }), { headers: { "Content-Type": "application/json" }, tags: { kind: "setup" } });
    if (login.status !== 200) fail(`login ${u.email}: ${login.status} ${login.body}`);
    const cookies = Object.values(login.cookies).map((list) => list[0]);
    const cookie = cookies.map((c) => `${c.name}=${c.value}`).join("; ");
    const csrf = (cookies.find((c) => /csrf/i.test(c.name)) || {}).value || "";
    const headers = { Cookie: cookie, "X-CSRF-Token": csrf, "Content-Type": "application/json" };
    const today = http.get(`${API}/today`, { headers, tags: { kind: "setup" } });
    if (today.status !== 200) fail(`today ${u.email}: ${today.status}`);
    const t = today.json();
    const open = t.my_check_ins.find((c) => c.pact_title === "Today: open") || t.my_check_ins[0];
    if (!open) fail(`no check-in for ${u.email}`);
    sessions.push({ headers, checkInId: open.check_in.id, pactIds: t.pacts.map((p) => p.pact_id) });
  }
  return { sessions };
}

const emptyJar = new http.CookieJar();
const get = (s, path, name) => {
  const res = http.get(`${API}${path}`, { headers: s.headers, jar: emptyJar, tags: { kind: "read", name } });
  check(res, { [`${name} 200`]: (r) => r.status === 200 });
};

export function read(data) {
  const s = data.sessions[exec.scenario.iterationInTest % data.sessions.length];
  const r = Math.random();
  if (r < 0.4) get(s, "/today", "today");
  else if (r < 0.55) get(s, "/pacts?limit=30", "pacts");
  else if (r < 0.75) get(s, `/pacts/${s.pactIds[Math.floor(Math.random() * s.pactIds.length)]}/ledger?limit=30`, "ledger");
  else if (r < 0.9) get(s, "/notifications?limit=20", "notifications");
  else get(s, "/review-queue?limit=20", "review-queue");
}

export function write(data) {
  const s = data.sessions[exec.scenario.iterationInTest % data.sessions.length];
  const words = `Latihan soal ${hex(6)} selesai, benar ${Math.floor(Math.random() * 40) + 10} dari 50 soal hari ini.`;
  const body = { body_doc: { type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: words }] }] }, attachment_ids: [] };
  const res = http.put(`${API}/check-ins/${s.checkInId}/proof`, JSON.stringify(body), {
    headers: Object.assign({}, s.headers, { "Idempotency-Key": uuid() }),
    jar: emptyJar,
    tags: { kind: "write", name: "submit-proof" },
  });
  check(res, { "submit-proof 2xx": (r) => r.status >= 200 && r.status < 300 });
}
