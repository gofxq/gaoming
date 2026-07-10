import { Button, Select, Tooltip } from "@douyinfe/semi-ui";
import {
  IconExit,
  IconHomeStroked,
  IconPulse,
  IconRefresh,
  IconServerStroked,
  IconUserGroup,
} from "@douyinfe/semi-icons";
import { useEffect, useMemo, useState } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
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
  const isStreaming = hostData.streamState === "实时推送中";

  useEffect(() => {
    document.body.classList.add("app-scroll");
    return () => document.body.classList.remove("app-scroll");
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
      <header className="topbar glass-panel">
        <div className="topbar-primary">
          <Link to={`/${tenantCode}`} className="brand" aria-label="高明监控首页">
            <span className="brand-mark"><IconPulse size="large" /></span>
            <span className="brand-copy">
              <strong>高明</strong>
              <small>GAOMING MONITOR</small>
            </span>
          </Link>

          <nav className="primary-nav" aria-label="主导航">
            <NavLink to={`/${tenantCode}`} end>
              <IconHomeStroked />
              <span>总览</span>
            </NavLink>
            {user?.role === "admin" ? (
              <NavLink to={`/${tenantCode}/users`}>
                <IconUserGroup />
                <span>用户</span>
              </NavLink>
            ) : null}
          </nav>

          <div className="topbar-context">
            <span className={`live-indicator ${isStreaming ? "is-live" : ""}`}>
              <i />
              {hostData.streamState}
            </span>
            <span className="tenant-badge">{tenantCode}</span>
          </div>
        </div>

        <div className="topbar-tools">
          <div className="tool-field host-picker">
            <IconServerStroked />
            <Select
              value={expandedHostUID || undefined}
              optionList={hostOptions}
              placeholder="定位主机"
              emptyContent="暂无主机"
              onChange={(value) => setExpandedHostUID(String(value || ""))}
            />
          </div>
          <Select
            value={selectedWindowSec}
            optionList={WINDOW_OPTIONS.map((option) => ({
              label: option.label,
              value: option.seconds,
            }))}
            className="window-picker"
            aria-label="趋势时间范围"
            onChange={(value) => setSelectedWindowSec(Number(value || 300))}
          />
          <Tooltip content="刷新主机列表">
            <Button
              className="icon-button"
              icon={<IconRefresh />}
              aria-label="刷新主机列表"
              onClick={() => void hostData.reloadHosts()}
            />
          </Tooltip>

          <span className="topbar-divider" />
          <div className="user-identity">
            <span className="user-avatar">{(user?.display_name || "访").slice(0, 1)}</span>
            <span>
              <strong>{user?.display_name || "访客"}</strong>
              <small>{user ? (user.role === "admin" ? "管理员" : "成员") : "只读访问"}</small>
            </span>
          </div>
          {user ? (
            <Tooltip content="退出登录">
              <Button
                className="icon-button quiet"
                icon={<IconExit />}
                aria-label="退出登录"
                onClick={() => void handleSignOut()}
              />
            </Tooltip>
          ) : null}
        </div>
      </header>

      <main className="content">
        <Outlet context={outletContext} />
      </main>
    </div>
  );
}
