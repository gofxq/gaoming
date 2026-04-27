package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/url"
	"time"
)

type WeChatOAuthClient interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (WeChatProfile, error)
	IsEnabled() bool
}

type HTTPWeChatOAuthClient struct {
	AppID       string
	AppSecret   string
	RedirectURL string
	Scope       string
	HTTPClient  *nethttp.Client
}

func NewWeChatOAuthClient(appID string, appSecret string, redirectURL string, scope string) *HTTPWeChatOAuthClient {
	return &HTTPWeChatOAuthClient{
		AppID:       appID,
		AppSecret:   appSecret,
		RedirectURL: redirectURL,
		Scope:       scope,
		HTTPClient: &nethttp.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *HTTPWeChatOAuthClient) IsEnabled() bool {
	return c != nil && c.AppID != "" && c.AppSecret != "" && c.RedirectURL != ""
}

func (c *HTTPWeChatOAuthClient) AuthURL(state string) string {
	values := url.Values{}
	values.Set("appid", c.AppID)
	values.Set("redirect_uri", c.RedirectURL)
	values.Set("response_type", "code")
	values.Set("scope", defaultString(c.Scope, "snsapi_login"))
	values.Set("state", state)
	return "https://open.weixin.qq.com/connect/qrconnect?" + values.Encode() + "#wechat_redirect"
}

func (c *HTTPWeChatOAuthClient) Exchange(ctx context.Context, code string) (WeChatProfile, error) {
	if !c.IsEnabled() {
		return WeChatProfile{}, errors.New("wechat login is not configured")
	}

	tokenValues := url.Values{}
	tokenValues.Set("appid", c.AppID)
	tokenValues.Set("secret", c.AppSecret)
	tokenValues.Set("code", code)
	tokenValues.Set("grant_type", "authorization_code")

	tokenURL := "https://api.weixin.qq.com/sns/oauth2/access_token?" + tokenValues.Encode()
	tokenResp := weChatTokenResponse{}
	if err := c.doJSON(ctx, tokenURL, &tokenResp); err != nil {
		return WeChatProfile{}, err
	}
	if tokenResp.ErrCode != 0 {
		return WeChatProfile{}, fmt.Errorf("wechat token exchange failed: %d %s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	profileValues := url.Values{}
	profileValues.Set("access_token", tokenResp.AccessToken)
	profileValues.Set("openid", tokenResp.OpenID)
	profileValues.Set("lang", "zh_CN")

	profileURL := "https://api.weixin.qq.com/sns/userinfo?" + profileValues.Encode()
	var profileResp weChatUserInfoResponse
	if err := c.doJSON(ctx, profileURL, &profileResp); err != nil {
		return WeChatProfile{}, err
	}
	if profileResp.ErrCode != 0 {
		return WeChatProfile{}, fmt.Errorf("wechat userinfo failed: %d %s", profileResp.ErrCode, profileResp.ErrMsg)
	}

	return WeChatProfile{
		OpenID:      defaultString(profileResp.OpenID, tokenResp.OpenID),
		UnionID:     defaultString(profileResp.UnionID, tokenResp.UnionID),
		Nickname:    profileResp.Nickname,
		AvatarURL:   profileResp.AvatarURL,
		Country:     profileResp.Country,
		Province:    profileResp.Province,
		City:        profileResp.City,
		Language:    profileResp.Language,
		Sex:         profileResp.Sex,
		AccessToken: tokenResp.AccessToken,
	}, nil
}

func (c *HTTPWeChatOAuthClient) doJSON(ctx context.Context, requestURL string, dest any) error {
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wechat request failed: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func defaultString(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

type weChatTokenResponse struct {
	AccessToken string `json:"access_token"`
	OpenID      string `json:"openid"`
	UnionID     string `json:"unionid"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type weChatAPIError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type weChatUserInfoResponse struct {
	OpenID    string `json:"openid"`
	UnionID   string `json:"unionid"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"headimgurl"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	Language  string `json:"language"`
	Sex       int    `json:"sex"`
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
}
