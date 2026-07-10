import { Button, Select, Tooltip } from "@douyinfe/semi-ui";
import {
  IconExit,
  IconHomeStroked,
  IconMoonStroked,
  IconPulse,
  IconRefresh,
  IconServerStroked,
  IconSunStroked,
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
  const [theme, setTheme] = useState<"light" | "dark">(() =>
    document.documentElement.dataset.theme === "dark" ? "dark" : "light",
  );
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
    document.documentElement.dataset.theme = theme;
    document.body.toggleAttribute("theme-mode", theme === "dark");
    window.localStorage.setItem("gaoming-theme", theme);
  }, [theme]);

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
        <div className="topbar-inner">
          <div className="topbar-primary">
            <Link to={`/${tenantCode}`} className="brand" aria-label="高明监控首页">
              <span className="brand-mark"><IconPulse /></span>
              <span className="brand-copy">
                <strong>高明</strong>
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
            <div className="window-segments" role="group" aria-label="趋势时间范围">
              {WINDOW_OPTIONS.map((option) => (
                <button
                  key={option.seconds}
                  type="button"
                  className={selectedWindowSec === option.seconds ? "active" : ""}
                  aria-pressed={selectedWindowSec === option.seconds}
                  onClick={() => setSelectedWindowSec(option.seconds)}
                >
                  {option.label}
                </button>
              ))}
            </div>
            <Tooltip content="刷新主机列表">
              <Button
                className="icon-button"
                icon={<IconRefresh />}
                aria-label="刷新主机列表"
                onClick={() => void hostData.reloadHosts()}
              />
            </Tooltip>
            <Tooltip content={theme === "dark" ? "切换浅色主题" : "切换深色主题"}>
              <Button
                className="icon-button quiet theme-toggle"
                icon={theme === "dark" ? <IconSunStroked /> : <IconMoonStroked />}
                aria-label={theme === "dark" ? "切换浅色主题" : "切换深色主题"}
                onClick={() => setTheme((current) => (current === "dark" ? "light" : "dark"))}
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

          <div className="mobile-tools">
            <Button
              className="icon-button"
              icon={<IconRefresh />}
              aria-label="刷新主机列表"
              onClick={() => void hostData.reloadHosts()}
            />
            <Button
              className="icon-button quiet"
              icon={theme === "dark" ? <IconSunStroked /> : <IconMoonStroked />}
              aria-label={theme === "dark" ? "切换浅色主题" : "切换深色主题"}
              onClick={() => setTheme((current) => (current === "dark" ? "light" : "dark"))}
            />
            {user ? (
              <Button
                className="icon-button quiet"
                icon={<IconExit />}
                aria-label="退出登录"
                onClick={() => void handleSignOut()}
              />
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
