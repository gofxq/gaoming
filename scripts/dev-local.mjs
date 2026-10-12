#!/usr/bin/env node
import { spawn } from "node:child_process";

const env = process.env;
const children = [];
let stopping = false;
let exitCode = 0;
let stopTimer;

function signalChild(child, signal) {
  if (!child.pid) return;
  try {
    if (process.platform === "win32") child.kill(signal);
    else process.kill(-child.pid, signal);
  } catch (error) {
    if (error.code !== "ESRCH") console.error(error.message);
  }
}

function stop(code) {
  if (stopping) return;
  stopping = true;
  exitCode = code;
  for (const child of children) signalChild(child, "SIGTERM");
  stopTimer = setTimeout(() => {
    for (const child of children) signalChild(child, "SIGKILL");
  }, 7000);
  // Keep the deadline alive for Air descendants even if the parent exits early.
  finish();
}

function finish() {
  if (stopping && children.every((child) => child.exitCode !== null || child.signalCode !== null || !child.pid)) {
    if (!env.AIR_CONFIG) clearTimeout(stopTimer);
    process.exitCode = exitCode;
  }
}

function start(name, command, args, options = {}) {
  const child = spawn(command, args, {
    stdio: "inherit",
    detached: process.platform !== "win32",
    ...options,
  });
  children.push(child);
  child.on("error", (error) => {
    console.error(`${name} failed to start: ${error.message}`);
    stop(1);
  });
  child.on("exit", (code, signal) => {
    if (!stopping) {
      console.error(`${name} stopped (${signal || code}); stopping local development.`);
      stop(code || 1);
    }
    finish();
  });
  return child;
}

process.on("SIGINT", () => stop(0));
process.on("SIGTERM", () => stop(0));

const httpPort = env.HTTP_PORT || "8080";
const webPort = env.WEB_PORT || "5173";
const base = `http://127.0.0.1:${httpPort}`;
start("backend", env.DEV_BACKEND || "./.tmp/local/bin/server", env.AIR_CONFIG ? ["-c", env.AIR_CONFIG] : ["-config", env.SERVER_CONFIG || "config/server.yml"], {
  env: { ...env, GAOMING_LOG_DIR: ".tmp/local/logs" },
});

async function launchFrontend() {
  const deadline = Date.now() + 15000;
  let healthy = false;
  while (!stopping && Date.now() < deadline) {
    try {
      const response = await fetch(`${base}/master/healthz`, { signal: AbortSignal.timeout(500) });
      healthy = response.ok;
      await response.body?.cancel();
      if (healthy) break;
    } catch { /* Backend is still starting. */ }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  if (stopping) return;
  if (!healthy) {
    console.error("Backend did not become healthy; check PostgreSQL, Redis and server configuration.");
    stop(1);
    return;
  }
  start("frontend", process.execPath, ["node_modules/vite/bin/vite.js", "--host", "0.0.0.0", "--port", webPort, "--strictPort"], {
    cwd: "web",
    env: { ...env, VITE_PROXY_TARGET: base },
  });
  console.log(`Local backend: ${base}; frontend: http://127.0.0.1:${webPort}/default`);
  console.log(`Agent: make run-agent (gRPC: 127.0.0.1:${env.GRPC_PORT || "8091"}). Ctrl-C stops backend and frontend.`);
}

launchFrontend().catch((error) => { console.error(error.message); stop(1); });
