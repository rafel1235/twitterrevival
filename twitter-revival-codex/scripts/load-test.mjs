import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { performance } from "node:perf_hooks";

const origin = process.env.LOAD_BASE_URL ?? "http://localhost:3000";
const url = new URL(origin);
if (
  !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) ||
  url.protocol !== "http:"
)
  throw new Error("Il test di carico è riservato alla beta HTTP locale.");
const count = Number(process.env.LOAD_USERS ?? 50);
const seconds = Number(process.env.LOAD_SECONDS ?? 30);
if (
  !Number.isInteger(count) ||
  count < 30 ||
  count > 50 ||
  !Number.isInteger(seconds) ||
  seconds < 15 ||
  seconds > 120
)
  throw new Error("Usare 30–50 utenti e 15–120 secondi.");
const run = Date.now().toString(36);
const cohort = `load-${run}`;
const accounts = [];
const timings = [];
const statuses = {};
const password = `Local load ${randomBytes(16).toString("hex")}`;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const headers = {
  Origin: origin,
  "X-CSRF-Protection": "1",
  "Content-Type": "application/json",
};
function invite(email) {
  const cli = process.env.E2E_API_CLI;
  const docker =
    process.platform === "win32"
      ? path.join(
          process.env.LOCALAPPDATA,
          "Programs/DockerDesktop/resources/bin/docker.exe",
        )
      : "docker";
  const args = [
    "-invite-email",
    email,
    "-invite-cohort",
    cohort,
    "-test-cohort",
  ];
  const result = execFileSync(
    cli ? path.resolve(cli) : docker,
    cli ? args : ["compose", "exec", "-T", "api", "/api", ...args],
    { stdio: "pipe", timeout: 20000 },
  );
  return JSON.parse(result.toString()).token;
}
async function request(route, account, method = "GET", body, retry = false) {
  for (let i = 0; i < (retry ? 4 : 1); i++) {
    const response = await fetch(origin + "/api/v1" + route, {
      method,
      headers: { ...headers, ...(account ? { Cookie: account.cookie } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(15000),
    });
    if (response.status !== 429 || !retry) return response;
    const delay = Number(response.headers.get("retry-after"));
    if (!Number.isFinite(delay) || delay < 1 || delay > 60)
      throw new Error("Retry-After non valido");
    await response.arrayBuffer();
    await sleep(delay * 1000 + 100);
  }
  throw new Error("Quota ancora esaurita dopo il retry");
}
async function requireStatus(response, status) {
  if (response.status !== status)
    throw new Error(`Risposta inattesa ${response.status}, atteso ${status}`);
  return response;
}
let report;
try {
  console.log(
    `Preparo ${count} utenti sintetici nella coorte ${cohort}; rispetto i limiti reali.`,
  );
  for (let i = 0; i < count; i++) {
    const handle = `l_${run}_${i}`;
    const email = `${handle}@example.test`;
    const response = await requireStatus(
      await request(
        "/auth/register",
        null,
        "POST",
        {
          handle,
          email,
          display_name: "Test carico locale",
          password,
          invite_token: invite(email),
        },
        true,
      ),
      201,
    );
    const user = (await response.json()).data.user;
    const cookie = response.headers.get("set-cookie")?.split(";")[0];
    if (!cookie) throw new Error("Sessione assente");
    accounts.push({ ...user, cookie });
  }
  const posts = [];
  for (let i = 0; i < 10; i++) {
    const response = await requireStatus(
      await request(
        "/posts",
        accounts[0],
        "POST",
        { body: `Prova carico ${run} · contenuto sintetico ${i}` },
        true,
      ),
      201,
    );
    posts.push((await response.json()).data.post.id);
  }
  for (const account of accounts.slice(1))
    await requireStatus(
      await request(
        `/users/${accounts[0].id}/follow`,
        account,
        "POST",
        {},
        true,
      ),
      204,
    );
  console.log(
    `Carico misto per ${seconds}s con ${count} utenti concorrenti, una richiesta per utente al secondo.`,
  );
  const started = performance.now();
  const until = started + seconds * 1000;
  let networkErrors = 0;
  await Promise.all(
    accounts.map(async (account, index) => {
      let iteration = 0;
      await sleep((index * 1000) / count);
      while (performance.now() < until) {
        const tick = performance.now();
        const choice = (iteration * 7 + index) % 100;
        const post = posts[(index + iteration) % posts.length];
        try {
          const response =
            choice < 60
              ? await request("/feed/home?limit=20", account)
              : choice < 88
                ? await request(`/posts/${post}`, account)
                : choice < 96
                  ? await request(`/users/${accounts[0].handle}`, account)
                  : await request(`/posts/${post}/likes`, account, "POST", {});
          await response.arrayBuffer();
          statuses[response.status] = (statuses[response.status] ?? 0) + 1;
        } catch {
          networkErrors++;
        }
        timings.push(performance.now() - tick);
        iteration++;
        await sleep(Math.max(0, 1000 - (performance.now() - tick)));
      }
    }),
  );
  const elapsed = (performance.now() - started) / 1000;
  timings.sort((a, b) => a - b);
  const percentile = (p) =>
    Math.round(
      timings[Math.min(timings.length - 1, Math.ceil(timings.length * p) - 1)] *
        100,
    ) / 100;
  const serverErrors = Object.entries(statuses)
    .filter(([status]) => Number(status) >= 500)
    .reduce((n, [, value]) => n + value, 0);
  const unexpected = Object.entries(statuses)
    .filter(([status]) => ![200, 429].includes(Number(status)))
    .reduce((n, [, value]) => n + value, 0);
  report = {
    timestamp_utc: new Date().toISOString(),
    environment:
      "Docker Desktop locale, nessuna garanzia di capacità in produzione",
    cohort_kind: "test",
    users: count,
    duration_seconds: Math.round(elapsed * 100) / 100,
    pacing_ms: 1000,
    requests: timings.length,
    requests_per_second: Math.round((timings.length / elapsed) * 100) / 100,
    latency_ms: {
      p50: percentile(0.5),
      p95: percentile(0.95),
      p99: percentile(0.99),
    },
    statuses,
    network_errors: networkErrors,
    thresholds: {
      p95_ms: 1500,
      p99_ms: 3000,
      max_rate_limited_fraction: 0.1,
      server_errors: 0,
    },
    passed:
      timings.length >= 100 &&
      percentile(0.95) < 1500 &&
      percentile(0.99) < 3000 &&
      !networkErrors &&
      !serverErrors &&
      !unexpected &&
      (statuses[429] ?? 0) / timings.length <= 0.1,
  };
} finally {
  let cleaned = 0;
  for (const account of accounts) {
    try {
      await requireStatus(
        await request(
          "/account",
          account,
          "DELETE",
          { password, confirmation: "ELIMINA" },
          true,
        ),
        204,
      );
      cleaned++;
    } catch {
      console.error(
        "Pulizia di un account sintetico non riuscita; consultare la coorte di test locale.",
      );
    }
  }
  if (report) {
    report.cleanup = {
      created_accounts: accounts.length,
      erased_accounts: cleaned,
    };
    report.passed &&= cleaned === accounts.length;
    await mkdir("docs/testing", { recursive: true });
    await writeFile(
      "docs/testing/load-sprint-6.json",
      JSON.stringify(report, null, 2) + "\n",
    );
    console.log(JSON.stringify(report, null, 2));
    if (!report.passed) process.exitCode = 1;
  }
}
