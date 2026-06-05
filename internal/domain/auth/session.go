package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"yunshu/internal/contracts"
)

var (
	// ErrInvalidLogin 表示登录参数缺失或不合法。
	ErrInvalidLogin = errors.New("invalid login")
	// ErrTokenNotFound 表示 token 不存在或已过期。
	ErrTokenNotFound = errors.New("token not found")
	// ErrSessionStoreUnavailable 表示会话存储未正确初始化。
	ErrSessionStoreUnavailable = errors.New("session store unavailable")
)

// LoginRequest 表示管理端登录请求。
type LoginRequest struct {
	Username    string
	Password    string
	MerchantID  string
	UserID      string
	RoleID      string
	DataScope   string
	Permissions []string
	Internal    bool
}

// LoginIdentity 表示通过账号密码校验后的登录身份。
type LoginIdentity struct {
	MerchantID string
	UserID     string
	RoleID     string
	DataScope  string
	Internal   bool
}

// LoginIdentityResolver 从数据库或其他持久化存储中验证账号密码并返回身份信息。
type LoginIdentityResolver interface {
	ResolveLoginIdentity(ctx context.Context, req LoginRequest) (LoginIdentity, error)
}

// LoginAccount 表示一个可用于默认登录或内存兜底的管理账号。
// 仅用于本地开发环境兜底，生产环境应通过数据库维护账号并禁用默认账号。
type LoginAccount struct {
	Username   string
	Password   string
	MerchantID string
	UserID     string
	RoleID     string
	DataScope  string
	Internal   bool
}

// DefaultLoginAccounts 返回默认管理账号。
// 这些账号既用于本地开发兜底，也用于数据库种子初始化。
// 仅在开发环境启用；生产环境通过环境变量 DISABLE_DEFAULT_ACCOUNTS=true 关闭。
func DefaultLoginAccounts() []LoginAccount {
	// 仅在开发环境启用默认账号，生产环境通过环境变量 DISABLE_DEFAULT_ACCOUNTS=true 关闭
	if os.Getenv("DISABLE_DEFAULT_ACCOUNTS") == "true" {
		return nil
	}
	return []LoginAccount{
		{Username: "admin", Password: "$2a$10$FE5nh0zJtomekwjO0JCXtOcG/X4qq5XebginTE8sDfAJMY24hgZiK", UserID: "9999", RoleID: "super_admin", Internal: true},
		{Username: "operator", Password: "$2a$10$8J16PjloeT2TWtso73rdlugL071u1QytlgNNRs05O3QS1r9PKf/N.", UserID: "1000", RoleID: "operate_lead", DataScope: "global"},
		{Username: "merchant", Password: "$2a$10$VVi8ExcLVxUeeEsd7J05peoAfGKJD/UTdxsepOnARtZv5nZv0xw6S", MerchantID: "1001", UserID: "2001", RoleID: "merchant_admin", DataScope: "merchant"},
	}
}

// AuthTicket 表示登录后返回的会话票据。
type AuthTicket struct {
	Token     string
	Tenant    contracts.TenantContext
	ExpiresAt time.Time
}

// SessionStore 定义 token 会话存储接口。
type SessionStore interface {
	Issue(ctx context.Context, tenant contracts.TenantContext, ttl time.Duration) (AuthTicket, error)
	Get(ctx context.Context, token string) (AuthTicket, bool)
	Revoke(ctx context.Context, token string) error
}

// LoginPermissionResolver 从数据库或其他配置源解析登录后的功能权限。
//
// 返回 ok=false 表示当前配置源没有命中，调用方可以继续使用静态兜底规则；
// 返回 ok=true 且权限为空表示命中但无授权，调用方应按无权限处理。
type LoginPermissionResolver interface {
	ResolveLoginPermissions(ctx context.Context, req LoginRequest) (permissions []string, ok bool, err error)
}

// RoutePermissionResolver 从数据库或其他配置源解析路由需要的功能权限。
type RoutePermissionResolver interface {
	RequiredPermissionForRequest(ctx context.Context, path, method string) (contracts.PermissionCode, bool, error)
}

// AuthService 承载管理端认证和会话逻辑。
type AuthService struct {
	Store            SessionStore
	IdentityResolver LoginIdentityResolver
	Permissions      LoginPermissionResolver
	TTL              time.Duration
	Logger           *slog.Logger
}

// Login 校验最基础的用户名密码并签发 token。
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (AuthTicket, error) {
	logger := s.logger()
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		logger.Warn("管理端登录参数无效", "username", req.Username)
		return AuthTicket{}, ErrInvalidLogin
	}
	tenant := contracts.TenantContext{
		MerchantID: strings.TrimSpace(req.MerchantID),
		UserID:     strings.TrimSpace(req.UserID),
		RoleID:     strings.TrimSpace(req.RoleID),
		DataScope:  strings.TrimSpace(req.DataScope),
		Internal:   req.Internal,
	}
	if s != nil && s.IdentityResolver != nil {
		identity, err := s.IdentityResolver.ResolveLoginIdentity(ctx, req)
		if err != nil {
			logger.Warn("管理端登录账号校验失败", "username", req.Username, "error", err.Error())
			return AuthTicket{}, err
		}
		tenant.MerchantID = strings.TrimSpace(identity.MerchantID)
		tenant.UserID = strings.TrimSpace(identity.UserID)
		tenant.RoleID = strings.TrimSpace(identity.RoleID)
		tenant.DataScope = strings.TrimSpace(identity.DataScope)
		tenant.Internal = identity.Internal
		req.MerchantID = tenant.MerchantID
		req.UserID = tenant.UserID
		req.RoleID = tenant.RoleID
		req.DataScope = tenant.DataScope
		req.Internal = tenant.Internal
	}
	permissions, err := s.resolvePermissions(ctx, req)
	if err != nil {
		logger.Error("管理端登录解析权限失败", "username", req.Username, "roleId", req.RoleID, "userId", req.UserID, "error", err.Error())
		return AuthTicket{}, err
	}
	tenant.Permissions = permissions
	ticket, err := s.Store.Issue(ctx, tenant, s.ttl())
	if err != nil {
		logger.Error("管理端登录签发 token 失败", "username", req.Username, "error", err.Error())
		return AuthTicket{}, err
	}
	tokenPrefix := ""
	if len(ticket.Token) > 8 {
		tokenPrefix = ticket.Token[:8] + "..."
	} else {
		tokenPrefix = "***"
	}
	logger.Info("管理端登录成功", "username", req.Username, "tokenPrefix", tokenPrefix, "expiresAt", ticket.ExpiresAt)
	return ticket, nil
}

// Token 查询 token 对应的租户上下文。
func (s *AuthService) Token(ctx context.Context, token string) (AuthTicket, bool) {
	return s.Store.Get(ctx, strings.TrimSpace(token))
}

// Logout 使 token 失效。
func (s *AuthService) Logout(ctx context.Context, token string) error {
	logger := s.logger()
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrInvalidLogin
	}
	tokenPrefix := ""
	if len(token) > 8 {
		tokenPrefix = token[:8] + "..."
	} else {
		tokenPrefix = "***"
	}
	if err := s.Store.Revoke(ctx, token); err != nil {
		logger.Warn("管理端注销 token 失败", "tokenPrefix", tokenPrefix, "error", err.Error())
		return err
	}
	logger.Info("管理端注销成功", "tokenPrefix", tokenPrefix)
	return nil
}

func (s *AuthService) ttl() time.Duration {
	if s != nil && s.TTL > 0 {
		return s.TTL
	}
	return 12 * time.Hour
}

func (s *AuthService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *AuthService) resolvePermissions(ctx context.Context, req LoginRequest) ([]string, error) {
	if req.Internal {
		return []string{string(contracts.PermissionConsoleAll)}, nil
	}
	if s != nil && s.Permissions != nil {
		permissions, ok, err := s.Permissions.ResolveLoginPermissions(ctx, req)
		if err != nil {
			return nil, err
		}
		if ok {
			return normalizePermissions(permissions), nil
		}
	}
	return resolvePermissions(req), nil
}

func resolvePermissions(req LoginRequest) []string {
	permissions := normalizePermissions(req.Permissions)
	if req.Internal {
		return []string{string(contracts.PermissionConsoleAll)}
	}
	if len(permissions) > 0 {
		return permissions
	}
	switch {
	case matchesRole(req.RoleID, "super_admin"):
		return []string{string(contracts.PermissionConsoleAll)}
	case matchesRole(req.RoleID, "operate_lead"):
		return operateLeadPermissions()
	case matchesRole(req.RoleID, "operate_staff"):
		return []string{
			string(contracts.PermissionOperateMerchantRead),
			string(contracts.PermissionOperateMerchantWrite),
			string(contracts.PermissionOperateAccountRead),
			string(contracts.PermissionOperateAccountWrite),
			string(contracts.PermissionOperateAccountToggle),
			string(contracts.PermissionOperateAccountPassword),
		}
	case matchesRole(req.RoleID, "merchant_admin"):
		return merchantAdminPermissions()
	case matchesRole(req.RoleID, "merchant_user"):
		return []string{
			string(contracts.PermissionMerchantBatchTaskRead),
			string(contracts.PermissionMerchantDialpadRead),
			string(contracts.PermissionMerchantCallRecordRead),
			string(contracts.PermissionMerchantAIFlowRead),
			string(contracts.PermissionMerchantSkillGroupRead),
			string(contracts.PermissionMerchantPhoneGroupRead),
		}
	default:
		return permissions
	}
}

func operateLeadPermissions() []string {
	return []string{
		string(contracts.PermissionOperateMerchantRead),
		string(contracts.PermissionOperateMerchantWrite),
		string(contracts.PermissionOperateMerchantDelete),
		string(contracts.PermissionOperateMerchantToggle),
		string(contracts.PermissionOperateAccountRead),
		string(contracts.PermissionOperateAccountWrite),
		string(contracts.PermissionOperateAccountDelete),
		string(contracts.PermissionOperateAccountToggle),
		string(contracts.PermissionOperateAccountPassword),
		string(contracts.PermissionOperateFreeSwitchRead),
		string(contracts.PermissionOperateGatewayRead),
		string(contracts.PermissionOperateGatewayWrite),
		string(contracts.PermissionOperateGatewayDelete),
		string(contracts.PermissionOperateGatewaySync),
		string(contracts.PermissionOperateChannelRead),
		string(contracts.PermissionOperateChannelWrite),
		string(contracts.PermissionOperateChannelDelete),
		string(contracts.PermissionOperateExtensionRead),
		string(contracts.PermissionOperateExtensionWrite),
		string(contracts.PermissionOperateExtensionDelete),
		string(contracts.PermissionOperateExtensionToggle),
		string(contracts.PermissionOperatePoolRead),
		string(contracts.PermissionOperatePoolWrite),
		string(contracts.PermissionOperatePoolDelete),
		string(contracts.PermissionOperatePhoneRead),
		string(contracts.PermissionOperatePhoneWrite),
		string(contracts.PermissionOperatePhoneDelete),
	}
}

func merchantAdminPermissions() []string {
	return []string{
		string(contracts.PermissionMerchantAccountRead),
		string(contracts.PermissionMerchantAccountWrite),
		string(contracts.PermissionMerchantAccountDelete),
		string(contracts.PermissionMerchantAccountToggle),
		string(contracts.PermissionMerchantAccountPassword),
		string(contracts.PermissionMerchantBatchTaskRead),
		string(contracts.PermissionMerchantBatchTaskWrite),
		string(contracts.PermissionMerchantBatchTaskDelete),
		string(contracts.PermissionMerchantBatchTaskToggle),
		string(contracts.PermissionMerchantDialpadRead),
		string(contracts.PermissionMerchantDialpadControl),
		string(contracts.PermissionMerchantCallRecordRead),
		string(contracts.PermissionMerchantAIFlowRead),
		string(contracts.PermissionMerchantAIFlowWrite),
		string(contracts.PermissionMerchantAIFlowDelete),
		string(contracts.PermissionMerchantAIFlowPrecheck),
		string(contracts.PermissionMerchantAIFlowPublish),
		string(contracts.PermissionMerchantSkillGroupRead),
		string(contracts.PermissionMerchantSkillGroupWrite),
		string(contracts.PermissionMerchantSkillGroupDelete),
		string(contracts.PermissionMerchantPhoneGroupRead),
		string(contracts.PermissionMerchantPhoneGroupWrite),
		string(contracts.PermissionMerchantPhoneGroupDelete),
	}
}

func normalizePermissions(permissions []string) []string {
	if len(permissions) == 0 {
		return nil
	}
	out := make([]string, 0, len(permissions))
	seen := map[string]struct{}{}
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		out = append(out, permission)
	}
	return out
}

func matchesRole(roleID string, candidates ...string) bool {
	roleID = strings.TrimSpace(strings.ToLower(roleID))
	if roleID == "" {
		return false
	}
	for _, candidate := range candidates {
		if roleID == candidate {
			return true
		}
	}
	return false
}

// MemorySessionStore 是本地开发和测试可用的内存 token 存储。
type MemorySessionStore struct {
	mu    sync.Mutex
	now   func() time.Time
	items map[string]AuthTicket
}

// NewMemorySessionStore 创建内存 token 存储。
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{now: time.Now, items: map[string]AuthTicket{}}
}

func (s *MemorySessionStore) Issue(_ context.Context, tenant contracts.TenantContext, ttl time.Duration) (AuthTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := randomToken()
	if err != nil {
		return AuthTicket{}, err
	}
	ticket := AuthTicket{Token: token, Tenant: tenant, ExpiresAt: s.now().UTC().Add(ttl)}
	s.items[token] = ticket
	return ticket, nil
}

func (s *MemorySessionStore) Get(_ context.Context, token string) (AuthTicket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, ok := s.items[token]
	if !ok {
		return AuthTicket{}, false
	}
	if !ticket.ExpiresAt.IsZero() && ticket.ExpiresAt.Before(s.now().UTC()) {
		delete(s.items, token)
		return AuthTicket{}, false
	}
	return ticket, true
}

func (s *MemorySessionStore) Revoke(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, token)
	return nil
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

const redisAuthSessionKeyPrefix = contracts.KeyConsoleAuthSessionPrefix
