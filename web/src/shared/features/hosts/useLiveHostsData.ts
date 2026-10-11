import { startTransition, useCallback, useEffect, useMemo, useState } from "react";
import {
  listToHostMap,
  mergeLatestHistory,
  normalizeHistoryMap,
  sortHosts,
  type HostDeletePayload,
  type HostHistoryMap,
  type HostMetricSnapshotPayload,
  type HostSnapshot,
  type HostSyncPayload,
  type HostUpsertPayload,
  type MetricSnapshot,
} from "./model";

const HISTORY_RETENTION_SEC = 3600;

function buildTenantScopedUrl(path: string, tenantCode: string) {
  const url = new URL(path, window.location.origin);
  url.searchParams.set("tenant", tenantCode);
  return url.toString();
}

export function useLiveHostsData(props: {
  apiBaseUrl: string;
  streamPath: string;
  tenantCode: string;
  onAccessCheck?: () => Promise<unknown>;
}) {
  const { apiBaseUrl, streamPath, tenantCode, onAccessCheck } = props;
  const [agents, setAgents] = useState<Record<string, HostSnapshot>>({});
  const [histories, setHistories] = useState<Record<string, HostHistoryMap>>({});
  const [metricSnapshots, setMetricSnapshots] = useState<Record<string, MetricSnapshot>>({});
  const [streamState, setStreamState] = useState("连接中");
  const [streamEventCount, setStreamEventCount] = useState(0);
  const [lastUpdated, setLastUpdated] = useState("");

  const hostsUrl = useMemo(
    () => buildTenantScopedUrl(`${apiBaseUrl}/hosts`, tenantCode),
    [apiBaseUrl, tenantCode],
  );
  const streamUrl = useMemo(
    () => buildTenantScopedUrl(streamPath, tenantCode),
    [streamPath, tenantCode],
  );

  const reloadHosts = useCallback(async () => {
    const response = await fetch(hostsUrl, {
      credentials: "include",
    });
    if (!response.ok) {
      if (response.status === 401 || response.status === 403) await onAccessCheck?.();
      throw new Error(`Failed to load hosts: ${response.status}`);
    }

    const payload = (await response.json()) as { items?: HostSnapshot[] };
    startTransition(() => {
      setAgents(listToHostMap(payload.items || []));
    });
  }, [hostsUrl, onAccessCheck]);

  const loadMetricSnapshot = useCallback(async (hostUID: string) => {
    if (!hostUID) {
      return;
    }
    const encodedHostUID = encodeURIComponent(hostUID);
    const url = buildTenantScopedUrl(`${apiBaseUrl}/hosts/${encodedHostUID}/metrics/latest`, tenantCode);
    const response = await fetch(url, { credentials: "include" });
    if (!response.ok) {
      if (response.status === 401 || response.status === 403) await onAccessCheck?.();
      throw new Error(`Failed to load metric snapshot: ${response.status}`);
    }

    const payload = (await response.json()) as HostMetricSnapshotPayload;
    setMetricSnapshots((current) => {
      if (!payload.snapshot) {
        const next = { ...current };
        delete next[hostUID];
        return next;
      }
      const currentCollectedAt = new Date(current[hostUID]?.collected_at || 0).getTime();
      const nextCollectedAt = new Date(payload.snapshot.collected_at).getTime();
      if (Number.isFinite(currentCollectedAt) && currentCollectedAt > nextCollectedAt) {
        return current;
      }
      return { ...current, [hostUID]: payload.snapshot };
    });
  }, [apiBaseUrl, tenantCode, onAccessCheck]);

  useEffect(() => {
    startTransition(() => {
      setAgents({});
      setHistories({});
      setMetricSnapshots({});
      setStreamState("连接中");
      setStreamEventCount(0);
      setLastUpdated("");
    });
  }, [hostsUrl, streamUrl, tenantCode]);

  useEffect(() => {
    let cancelled = false;

    reloadHosts().catch(() => {
      if (!cancelled) {
        setStreamState("等待实时流");
      }
    });

    return () => {
      cancelled = true;
    };
  }, [reloadHosts]);

  useEffect(() => {
    const stream = new EventSource(streamUrl, {
      withCredentials: true,
    });

    stream.addEventListener("open", () => {
      setStreamState("实时推送中");
    });

    stream.addEventListener("error", () => {
      setStreamState("重连中");
      void onAccessCheck?.();
    });

    stream.addEventListener("sync", (event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as HostSyncPayload;
      startTransition(() => {
        setStreamEventCount((current) => current + 1);
        setAgents(listToHostMap(payload.items || []));
        setHistories(() => {
          const next: Record<string, HostHistoryMap> = {};
          Object.entries(payload.histories || {}).forEach(([hostUID, history]) => {
            next[hostUID] = normalizeHistoryMap(history, HISTORY_RETENTION_SEC);
          });
          Object.entries(payload.latest || {}).forEach(([hostUID, latestPoints]) => {
            next[hostUID] = mergeLatestHistory(
              next[hostUID],
              latestPoints,
              HISTORY_RETENTION_SEC,
            );
          });
          return next;
        });
        setLastUpdated(payload.server_time || "");
      });
    });

    stream.addEventListener("host_upsert", (event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as HostUpsertPayload;
      if (!payload.item?.host_uid) {
        return;
      }

      startTransition(() => {
        setStreamEventCount((current) => current + 1);
        setAgents((current) => ({
          ...current,
          [payload.item!.host_uid]: payload.item!,
        }));
        setHistories((current) => ({
          ...current,
          [payload.item!.host_uid]: mergeLatestHistory(
            current[payload.item!.host_uid],
            payload.latest,
            HISTORY_RETENTION_SEC,
          ),
        }));
        setLastUpdated(payload.server_time || "");
      });
    });

    stream.addEventListener("host_delete", (event) => {
      const payload = JSON.parse((event as MessageEvent<string>).data) as HostDeletePayload;
      if (!payload.host_uid) {
        return;
      }

      startTransition(() => {
        setStreamEventCount((current) => current + 1);
        setAgents((current) => {
          const next = { ...current };
          delete next[payload.host_uid!];
          return next;
        });
        setHistories((current) => {
          const next = { ...current };
          delete next[payload.host_uid!];
          return next;
        });
        setMetricSnapshots((current) => {
          const next = { ...current };
          delete next[payload.host_uid!];
          return next;
        });
        setLastUpdated(payload.server_time || "");
      });
    });

    return () => {
      stream.close();
    };
  }, [streamUrl, onAccessCheck]);

  const sortedHosts = useMemo(() => sortHosts(Object.values(agents)), [agents]);

  return {
    agents,
    histories,
    lastUpdated,
    loadMetricSnapshot,
    metricSnapshots,
    reloadHosts,
    sortedHosts,
    streamEventCount,
    streamState,
  };
}
