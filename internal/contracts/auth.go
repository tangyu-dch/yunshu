package contracts

import "time"

// AuthLoginReq 表示管理端登录请求。
type AuthLoginReq struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	MerchantID  string   `json:"merchantId,omitempty"`
	UserID      string   `json:"userId,omitempty"`
	RoleID      string   `json:"roleId,omitempty"`
	DataScope   string   `json:"dataScope,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Internal    bool     `json:"internal"`
}

// AuthLoginResp 表示登录成功后的 token 载荷。
type AuthLoginResp struct {
	Token     string        `json:"token"`
	ExpiresAt time.Time     `json:"expiresAt"`
	Tenant    TenantContext `json:"tenant"`
}
