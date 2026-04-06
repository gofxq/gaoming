import { startTransition, useEffect, useMemo, useState } from "react";
import {
  listToHostMap,
  mergeLatestHistory,
  normalizeHistoryMap,
  sortHosts,
  type HostDeletePayload,
  type HostHistoryMap,
  type HostSnapshot,
  type HostSyncPayload,
  type HostUpsertPayload,
} from "./dashboard";

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
}) {
  const { apiBaseUrl, streamPath, tenantCode } = props;
  const [agents, setAgents] = useState<Record<string, HostSnapshot>>({});
  const [histories, setHistories] = useState<Record<string, HostHistoryMap>>({});
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

  useEffect(() => {
    startTransition(() => {
      setAgents({});
      setHistories({});
      setStreamState("连接中");
      setStreamEventCount(0);
      setLastUpdated("");
    });
  }, [hostsUrl, streamUrl, tenantCode]);

  useEffect(() => {
    let cancelled = false;

    async function loadHosts() {
      try {
        const response = await fetch(hostsUrl);
        if (!response.ok) {
          return;
        }

        const payload = (await response.json()) as { items?: HostSnapshot[] };
        if (cancelled) {
          return;
        }

        startTransition(() => {
          setAgents(listToHostMap(payload.items || []));
        });
      } catch {
        // The dashboard can still bootstrap from SSE sync.
      }
    }

    void loadHosts();

    return () => {
      cancelled = true;
    };
  }, [hostsUrl]);

  useEffect(() => {
    const stream = new EventSource(streamUrl);

    stream.addEventListener("open", () => {
      setStreamState("实时推送中");
    });

    stream.addEventListener("error", () => {
      setStreamState("重连中");
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
        setLastUpdated(payload.server_time || "");
      });
    });

    return () => {
      stream.close();
    };
  }, [streamUrl]);

  const sortedHosts = useMemo(() => sortHosts(Object.values(agents)), [agents]);

  return {
    agents,
    histories,
    lastUpdated,
    sortedHosts,
    streamEventCount,
    streamState,
  };
}
