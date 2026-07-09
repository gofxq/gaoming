import { Link, Outlet, useNavigate } from "react-router-dom";
import { Button, Select, Space } from "@douyinfe/semi-ui";
import { useEffect, useMemo, useState } from "react";
import { useAppConfig } from "../../app/providers/AppConfigProvider";
import { useAuth } from "../../app/providers/AuthProvider";
import { useTenant } from "../../app/providers/TenantProvider";
import { WINDOW_OPTIONS } from "../../features/hosts/model";
import { useLiveHostsData } from "../../features/hosts/useLiveHostsData";

export type ShellOutletContext = ReturnType<typeof useLiveHostsData> & {
  selectedWindowSec: number;
  setSelectedWindowSec: (value: number) => void;
  expandedHostUID: string;
  setExpandedHostUID: (value: string | ((current: string) => string)) => void;
};

export function Shell() {
  const { config } = useAppConfig();
  const { tenantCode } = useTenant();
  const { user, signOut } = useAuth();
  const navigate = useNavigate();
  const [selectedWindowSec, setSelectedWindowSec] = useState(300);
  const [expandedHostUID, setExpandedHostUID] = useState("");
  const hostData = useLiveHostsData({
    apiBaseUrl: config.apiBaseUrl,
    streamPath: config.streamPath,
    tenantCode,
  });

  const totalCount = hostData.sortedHosts.length;
  const onlineCount = hostData.sortedHosts.filter((host) => host.overall_state !== 4).length;
  const warningCount = hostData.sortedHosts.filter((host) => host.overall_state === 2 || host.overall_state === 3).length;
  const offlineCount = hostData.sortedHosts.filter((host) => host.overall_state === 4).length;
  const outletContext: ShellOutletContext = {
    ...hostData,
    selectedWindowSec,
    setSelectedWindowSec,
    expandedHostUID,
    setExpandedHostUID,
  };
  const hostOptions = useMemo(
    () =>
      hostData.sortedHosts.map((host) => ({
        label: host.hostname || host.host_uid,
        value: host.host_uid,
      })),
    [hostData.sortedHosts],
  );

  useEffect(() => {
    document.body.classList.add("desktop-scroll");
    return () => {
      document.body.classList.remove("desktop-scroll");
    };
  }, []);

  useEffect(() => {
    setExpandedHostUID("");
  }, [tenantCode]);

  useEffect(() => {
    if (expandedHostUID && !hostData.sortedHosts.some((host) => host.host_uid === expandedHostUID)) {
      setExpandedHostUID("");
    }
  }, [expandedHostUID, hostData.sortedHosts]);

  async function handleSignOut() {
    await signOut();
    navigate(`/${tenantCode}/login`, { replace: true });
  }

  return (
    <div className="shell">
      <header className="topbar">
        <div>
          <div className="eyebrow">Gaoming Web</div>
          <h1 className="headline">监控看板</h1>
          <div className="topbar-stream-line">
            Tenant {tenantCode} · {hostData.streamState} · {hostData.streamEventCount} events
          </div>
          <div className="topbar-host-stats" aria-label="host summary">
            <span className="topbar-stat-pill blue">
              <span>主机</span>
              <strong>{totalCount}</strong>
            </span>
            <span className="topbar-stat-pill green">
              <span>在线</span>
              <strong>{onlineCount}</strong>
            </span>
            <span className="topbar-stat-pill orange">
              <span>异常</span>
              <strong>{warningCount}</strong>
            </span>
            <span className="topbar-stat-pill red">
              <span>离线</span>
              <strong>{offlineCount}</strong>
            </span>
          </div>
        </div>
        <div className="shell-actions">
          <Space className="topbar-controls">
            <Select
              value={expandedHostUID}
              optionList={hostOptions}
              placeholder="展开主机"
              className="topbar-action topbar-host-select"
              onChange={(value) => setExpandedHostUID(String(value || ""))}
            />
            <Select
              value={selectedWindowSec}
              optionList={WINDOW_OPTIONS.map((option) => ({
                label: option.label,
                value: option.seconds,
              }))}
              className="topbar-action topbar-window-select"
              onChange={(value) => setSelectedWindowSec(Number(value || 300))}
            />
            <Button className="topbar-action topbar-refresh" onClick={() => void hostData.reloadHosts()}>
              刷新
            </Button>
          </Space>
          <nav className="shell-badges">
            <Link to={`/${tenantCode}`} className="meta-pill topbar-action-link">
              Dashboard
            </Link>
            {user?.role === "admin" ? (
              <Link to={`/${tenantCode}/users`} className="meta-pill topbar-action-link">
                Users
              </Link>
            ) : null}
            <span className="meta-pill">Tenant: {tenantCode}</span>
          </nav>
          <div className="shell-user">
            <span className="meta-pill">{user?.display_name || "未登录"}</span>
            {user ? (
              <>
                <span className="meta-pill">{user.role === "admin" ? "管理员" : "成员"}</span>
                <button
                  type="button"
                  className="meta-pill meta-pill-button topbar-action-link"
                  onClick={() => void handleSignOut()}
                >
                  退出登录
                </button>
              </>
            ) : null}
          </div>
        </div>
      </header>

      <main className="content">
        <Outlet context={outletContext} />
      </main>
    </div>
  );
}
