import { useEffect, useRef, useState, type FormEvent } from "react";
import { useAuth } from "../../shared/features/auth/AuthProvider";
import { useAppConfig } from "../../shared/features/config/AppConfigProvider";
import { writeAuthorizedJson } from "../../shared/features/auth/api";
import { requestJson } from "../../shared/lib/http";

type Share = { id: string; title: string; host_uids: string[]; fields: string[]; expires_at: string; revoked_at?: string };
type Host = { host_uid: string; hostname?: string };
const fieldLabels: Record<string, string> = { hostname: "主机名称", status: "健康状态", metrics: "固定摘要指标" };

export function SharesManagePage() {
  const { config } = useAppConfig();
  const { user } = useAuth();
  const [shares, setShares] = useState<Share[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [hostUIDs, setHostUIDs] = useState<string[]>([]);
  const [fields, setFields] = useState<string[]>(["status"]);
  const [title, setTitle] = useState("");
  const [hours, setHours] = useState(24);
  const [createdLink, setCreatedLink] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const requestRevision = useRef(0);

  async function reload(signal?: AbortSignal) {
    const current = ++requestRevision.current;
    const [items, hostItems] = await Promise.all([
      requestJson<{ items: Share[] }>(`${config.apiBaseUrl}/shares`, { credentials: "include", cache: "no-store", signal }),
      requestJson<{ items: Host[] }>(`${config.apiBaseUrl}/hosts`, { credentials: "include", cache: "no-store", signal }),
    ]);
    if (current === requestRevision.current) { setShares(items.items); setHosts(hostItems.items); }
  }
  useEffect(() => {
    const controller = new AbortController();
    void reload(controller.signal).catch((cause) => { if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : "分享列表读取失败"); });
    return () => { requestRevision.current++; controller.abort(); };
  }, [config.apiBaseUrl, user?.id, user?.tenant_code]);

  async function create(event: FormEvent) {
    event.preventDefault(); if (busy) return;
    if (hostUIDs.length === 0 || fields.length === 0) { setError("请选择至少一台主机和一个允许字段。"); return; }
    setBusy(true); setError(""); setMessage(""); setCreatedLink("");
    try {
      const result = await writeAuthorizedJson<Share & { token: string }>(config.apiBaseUrl, "/shares", { title, host_uids: hostUIDs, fields, ttl_seconds: hours * 3600 });
      const link = new URL("/share", window.location.origin); link.hash = `token=${result.token}`;
      setCreatedLink(link.toString()); setMessage("分享已创建。链接包含只读访问凭据，仅在此显示一次，请仅发送给预期接收者。");
      await reload();
    } catch (cause) { setError(cause instanceof Error ? cause.message : "创建失败，请重试"); }
    finally { setBusy(false); }
  }
  async function revoke(share: Share) {
    if (busy) return; setBusy(true); setError(""); setMessage("");
    try { await writeAuthorizedJson<void>(config.apiBaseUrl, `/shares/${encodeURIComponent(share.id)}`, {}, "DELETE"); setCreatedLink(""); await reload(); setMessage("分享已撤销，新请求被拒绝，已连接的查看页面将在权限重验时停止接收。"); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "撤销失败，请重试"); }
    finally { setBusy(false); }
  }
  const toggle = (current: string[], value: string, selected: boolean) => selected ? [...current, value] : current.filter((item) => item !== value);

  return <div className="dashboard"><section className="panel shares-panel">
    <h1>受限分享管理</h1><p>分享仅限当前空间选定的主机，稳定主机标识始终包含；IP、标签、设备维度和采集器明细不会公开。每个空间最多保留 100 条分享记录。</p>
    {error ? <div className="auth-error" role="alert">{error}</div> : null}
    {message ? <p role="status">{message}</p> : null}
    <form className="auth-form" onSubmit={(event) => void create(event)}>
      <label>分享说明<input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={128} required disabled={busy} /></label>
      <fieldset><legend>允许查看的主机（最多 100 台）</legend>{hosts.length === 0 ? <p>尚无可分享主机，请先接入主机。</p> : hosts.map((host) => <label key={host.host_uid}><input type="checkbox" checked={hostUIDs.includes(host.host_uid)} disabled={busy} onChange={(event) => setHostUIDs((current) => toggle(current, host.host_uid, event.target.checked))} />{host.hostname || host.host_uid}</label>)}</fieldset>
      <fieldset><legend>允许字段</legend>{Object.entries(fieldLabels).map(([field, label]) => <label key={field}><input type="checkbox" checked={fields.includes(field)} disabled={busy} onChange={(event) => setFields((current) => toggle(current, field, event.target.checked))} />{label}</label>)}</fieldset>
      <label>有效期<select value={hours} onChange={(event) => setHours(Number(event.target.value))} disabled={busy}><option value={1}>1 小时</option><option value={24}>24 小时</option><option value={168}>7 天</option></select></label>
      <button type="submit" disabled={busy || hosts.length === 0}>{busy ? "正在处理…" : "创建分享"}</button>
    </form>
    {createdLink ? <label className="share-link">一次性显示的分享链接<input readOnly value={createdLink} onFocus={(event) => event.target.select()} /><button type="button" onClick={() => { if (!navigator.clipboard) { setError("复制不可用，请选中链接手动复制。"); return; } void navigator.clipboard.writeText(createdLink).then(() => setMessage("分享链接已复制。"), () => setError("复制不可用，请选中链接手动复制。")); }}>复制链接</button></label> : null}
    <h2>已有分享</h2>{shares.length === 0 ? <p>尚无分享记录。</p> : shares.map((share) => <article className="shared-host" key={share.id}><h3>{share.title}</h3><p>{share.host_uids.length} 台主机 · {share.fields.map((field) => fieldLabels[field]).join("、")}</p><p>到期：{new Date(share.expires_at).toLocaleString()} · {share.revoked_at ? "已撤销" : new Date(share.expires_at).getTime() <= Date.now() ? "已过期" : "有效"}</p><button type="button" disabled={busy || Boolean(share.revoked_at)} onClick={() => void revoke(share)}>撤销分享</button></article>)}
  </section></div>;
}
