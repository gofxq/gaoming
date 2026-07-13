import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAppConfig } from "../../shared/features/config/AppConfigProvider";
import { useAuth } from "../../shared/features/auth/AuthProvider";
import { requestJson } from "../../shared/lib/http";

type UserRecord = {
  id: number;
  tenant_code: string;
  display_name: string;
  avatar_url?: string;
  role: "admin" | "member";
  status: "active" | "disabled";
  last_login_at?: string;
  created_at: string;
  updated_at: string;
  provider?: string;
  provider_user_id?: string;
};

type UsersPayload = {
  items?: UserRecord[];
};

type UserPatch = Partial<Pick<UserRecord, "display_name" | "role" | "status">>;

function usersQueryKey(apiBaseUrl: string) {
  return ["admin-users", apiBaseUrl] as const;
}

export function UsersPage() {
  const { config } = useAppConfig();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [savingID, setSavingID] = useState<number | null>(null);

  const query = useQuery({
    queryKey: usersQueryKey(config.apiBaseUrl),
    queryFn: async () => {
      const payload = await requestJson<UsersPayload>(`${config.apiBaseUrl}/admin/users`, {
        credentials: "include",
      });
      return payload.items || [];
    },
  });

  const mutation = useMutation({
    mutationFn: async ({ target, patch }: { target: UserRecord; patch: UserPatch }) =>
      requestJson<UserRecord>(`${config.apiBaseUrl}/admin/users/${target.id}`, {
        method: "PATCH",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(patch),
      }),
    onMutate: ({ target }) => {
      setSavingID(target.id);
    },
    onSuccess: (nextUser) => {
      queryClient.setQueryData<UserRecord[]>(usersQueryKey(config.apiBaseUrl), (current) =>
        (current || []).map((item) => (item.id === nextUser.id ? nextUser : item)),
      );
    },
    onSettled: () => {
      setSavingID(null);
    },
  });

  const items = query.data || [];
  const errorMessage =
    (query.error instanceof Error && query.error.message) ||
    (mutation.error instanceof Error && mutation.error.message) ||
    "";

  async function updateUser(
    target: UserRecord,
    patch: Partial<Pick<UserRecord, "display_name" | "role" | "status">>,
  ) {
    await mutation.mutateAsync({ target, patch });
  }

  return (
    <div className="dashboard">
      <section className="panel">
        <div className="panel-head">
          <div>
            <div className="eyebrow">成员与权限</div>
            <h2>用户管理</h2>
          </div>
          <p className="panel-note">管理员可查看用户，并调整角色与启用状态。</p>
        </div>

        <div className="hero-meta">
          <span className="meta-pill">当前管理员: {user?.display_name || "--"}</span>
          <span className="meta-pill">共 {items.length} 位用户</span>
        </div>

        {errorMessage ? <div className="auth-error">{errorMessage}</div> : null}
        {query.isLoading ? <div className="empty-state">正在加载用户...</div> : null}

        {!query.isLoading ? (
          <div className="user-table">
            <div className="user-table-head">
              <span>用户</span>
              <span>身份</span>
              <span>状态</span>
              <span>最近登录</span>
              <span>操作</span>
            </div>
            {items.map((item) => (
              <div key={item.id} className="user-row">
                <div className="user-cell user-profile">
                  {item.avatar_url ? (
                    <img src={item.avatar_url} alt={item.display_name} className="user-avatar" />
                  ) : null}
                  <div>
                    <strong>{item.display_name}</strong>
                    <span>
                      {item.provider || "本地用户"} · {item.provider_user_id || "手动创建"}
                    </span>
                  </div>
                </div>
                <div className="user-cell">
                  <label>
                    <span className="sr-only">角色</span>
                    <select
                      value={item.role}
                      disabled={savingID === item.id}
                      onChange={(event) =>
                        void updateUser(item, { role: event.target.value as UserRecord["role"] })
                      }
                    >
                      <option value="admin">管理员</option>
                      <option value="member">成员</option>
                    </select>
                  </label>
                </div>
                <div className="user-cell">
                  <label>
                    <span className="sr-only">状态</span>
                    <select
                      value={item.status}
                      disabled={savingID === item.id}
                      onChange={(event) =>
                        void updateUser(item, { status: event.target.value as UserRecord["status"] })
                      }
                    >
                      <option value="active">启用</option>
                      <option value="disabled">禁用</option>
                    </select>
                  </label>
                </div>
                <div className="user-cell">
                  <span>{item.last_login_at ? new Date(item.last_login_at).toLocaleString() : "未登录"}</span>
                </div>
                <div className="user-cell">
                  <button
                    type="button"
                    className="chip"
                    disabled={savingID === item.id}
                    onClick={() => void updateUser(item, { display_name: item.display_name })}
                  >
                    {savingID === item.id ? "保存中..." : "保存"}
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : null}
      </section>
    </div>
  );
}
