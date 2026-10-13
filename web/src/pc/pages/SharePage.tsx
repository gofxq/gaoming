import { useEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { useAppConfig } from "../../shared/features/config/AppConfigProvider";
import { HttpError, requestJson } from "../../shared/lib/http";
import { consumeShareFragment } from "../../shared/features/shares/entry";
import { AppearanceControls } from "../components/appearance/AppearanceControls";

let entryHadFragment = window.location.pathname === "/share" && Boolean(window.location.hash);
let pendingEntryToken = consumeShareFragment();
type SharedHost = { host_uid: string; hostname?: string; overall_state?: number; cpu_usage_pct?: number; mem_used_pct?: number; last_agent_seen_at?: string; last_metric_at?: string };
const statuses = ["状态未知", "正常", "需要关注", "严重异常", "离线", "维护中", "已停用"];

export function SharePage() {
  const { config } = useAppConfig();
  const location = useLocation();
  const demo = location.pathname === "/demo";
  const base = `${config.apiBaseUrl}/public/${demo ? "demo" : "shares"}`;
  const [hosts, setHosts] = useState<SharedHost[]>([]);
  const [fields, setFields] = useState<string[]>([]);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  const entryToken = useRef(pendingEntryToken);
  const credentialedEntry = useRef(entryHadFragment);
  const rejectedEntry = useRef(entryHadFragment && !pendingEntryToken);
  const exchange = useRef<Promise<unknown> | null>(null);
  const exchangeController = useRef<AbortController | null>(null);
  const mounted = useRef(false);

  useEffect(() => {
    mounted.current = true;
    let cancelled = false;
    let stream: EventSource | null = null;
    const controller = new AbortController();
    setBusy(true); setError(""); setHosts([]); setFields([]);
    async function load() {
      if (!demo && rejectedEntry.current) throw new HttpError(401, "share unavailable");
      if (!demo && entryToken.current && !exchange.current) {
        exchangeController.current = new AbortController();
        exchange.current = requestJson(`${base}/exchange`, { method: "POST", credentials: "include", signal: exchangeController.current.signal, headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token: entryToken.current }) })
          .then((result) => { entryToken.current = ""; return result; });
        pendingEntryToken = ""; entryHadFragment = false;
      }
      if (!demo && exchange.current) await exchange.current;
      if (cancelled) return;
      const info = await requestJson<{ fields: string[] }>(`${base}/session`, { credentials: "include", cache: "no-store", signal: controller.signal });
      const payload = await requestJson<{ items: SharedHost[] }>(`${base}/hosts`, { credentials: "include", cache: "no-store", signal: controller.signal });
      if (cancelled) return;
      setFields(info.fields); setHosts(payload.items); setBusy(false);
      stream = new EventSource(`${base}/stream/hosts`, { withCredentials: true });
      const failClosed = () => {
        stream?.close();
        if (!cancelled) { setHosts([]); setError("实时连接已中断，已隐藏分享数据。请重新验证访问权限。"); }
      };
      stream.addEventListener("error", failClosed);
      stream.addEventListener("sync", (event) => {
        try { const next = JSON.parse((event as MessageEvent<string>).data) as { items: SharedHost[] }; if (!Array.isArray(next.items)) throw new Error(); if (!cancelled) setHosts(next.items); } catch { failClosed(); }
      });
      stream.addEventListener("host_upsert", (event) => {
        try { const next = JSON.parse((event as MessageEvent<string>).data) as { item: SharedHost }; if (!next.item?.host_uid) throw new Error(); if (!cancelled) setHosts((current) => current.map((host) => host.host_uid === next.item.host_uid ? next.item : host)); } catch { failClosed(); }
      });
    }
    void load().catch((cause) => {
      if (cancelled) return;
      exchange.current = null;
      if (cause instanceof HttpError && (cause.status === 401 || cause.status === 403)) {
        entryToken.current = "";
        if (credentialedEntry.current) rejectedEntry.current = true;
      }
      setBusy(false); setHosts([]);
      setError(cause instanceof HttpError && (cause.status === 401 || cause.status === 403) ? (demo ? "演示入口未开放或当前查看范围不可用。" : "分享已过期、被撤销或不可用。请向分享者索取有效链接。") : "无法读取分享，请检查网络后重试。");
    });
    return () => {
      cancelled = true; controller.abort(); stream?.close(); mounted.current = false;
      // React's development effect replay reinstates mounted synchronously.
      // Clear credentials/abort exchange when the page actually goes away.
      queueMicrotask(() => {
        if (!mounted.current) { entryToken.current = ""; exchangeController.current?.abort(); exchange.current = null; }
      });
    };
  }, [base, demo, revision]);

  return <div className="auth-page"><AppearanceControls className="auth-appearance-controls" /><section className="auth-card shared-card">
    <div className="eyebrow">{demo ? "公开演示" : "受限分享"}</div><h1>{demo ? "演示主机" : "只读主机视图"}</h1>
    <p className="auth-copy">仅显示明确授权的主机和字段；分享不授予管理权限。名称、状态或摘要指标未被分享时不会展示。</p>
    {busy ? <p role="status">正在验证查看范围…</p> : null}
    {error ? <div className="auth-error" role="alert">{error}</div> : null}
    {!busy && !error && hosts.length === 0 ? <p>当前授权范围内没有可用主机。</p> : null}
    {hosts.map((host) => <article className="shared-host" key={host.host_uid}>
      <h2>{host.hostname || host.host_uid}</h2>
      <p>{fields.includes("status") ? statuses[host.overall_state ?? 0] || "状态未知" : "未分享状态"}</p>
      {fields.includes("metrics") ? <p>{typeof host.cpu_usage_pct === "number" ? `CPU ${host.cpu_usage_pct.toFixed(1)}% · 内存 ${host.mem_used_pct?.toFixed(1) ?? "--"}%` : "尚无有效摘要指标"}</p> : <p>未分享资源指标</p>}
      {host.last_agent_seen_at ? <small>最后上报：{new Date(host.last_agent_seen_at).toLocaleString()}</small> : null}
    </article>)}
    <button type="button" onClick={() => setRevision((value) => value + 1)} disabled={busy}>重新验证</button>
    <Link to="/default/login">返回登录</Link>
  </section></div>;
}
