import { useMemo, useState } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "../../app/providers/AuthProvider";
import { useTenant } from "../../app/providers/TenantProvider";

export function LoginPage() {
    const location = useLocation();
    const { tenantCode } = useTenant();
    const { authenticated, beginWeChatLogin, initializing, weChatEnabled, user } = useAuth();
    const [submitting, setSubmitting] = useState(false);
    const [error, setError] = useState("");
    const returnTo = useMemo(() => {
        const next =
            new URLSearchParams(location.search).get("return_to") ||
            `/${tenantCode}`;
        return next.startsWith("/") ? next : `/${tenantCode}`;
    }, [location.search, tenantCode]);

    if (!initializing && authenticated && user) {
        return <Navigate to={returnTo || `/${user.tenant_code}`} replace />;
    }

    async function handleLogin() {
        setSubmitting(true);
        setError("");
        try {
            await beginWeChatLogin(returnTo);
        } catch (nextError) {
            setError(nextError instanceof Error ? nextError.message : "微信登录失败");
            setSubmitting(false);
        }
    }

    return (
        <div className="auth-page">
            <section className="auth-card">
                <div className="eyebrow">Gaoming Auth</div>
                <h1>微信登录</h1>
                <p className="auth-copy">
                    通过微信扫码登录监控后台，首次登录会自动创建租户内用户，首个用户默认授予管理员权限。
                </p>
                <div className="auth-meta">
                    <span className="meta-pill">Tenant: {tenantCode}</span>
                    <span className="meta-pill">{weChatEnabled ? "WeChat Ready" : "WeChat Disabled"}</span>
                </div>
                <button
                    type="button"
                    className="auth-primary"
                    disabled={!weChatEnabled || submitting || initializing}
                    onClick={() => void handleLogin()}
                >
                    {submitting ? "跳转中..." : "使用微信登录"}
                </button>
                {!weChatEnabled ? (
                    <div className="auth-error">后端尚未配置微信 AppID / Secret / 回调地址。</div>
                ) : null}
                {error ? <div className="auth-error">{error}</div> : null}
            </section>
        </div>
    );
}
