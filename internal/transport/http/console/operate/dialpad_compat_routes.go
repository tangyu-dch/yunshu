package operate

import (
	"crypto/aes"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/config"
	"yunshu/internal/infra/resource"
	"yunshu/internal/infra/storage"
)

// Keys and Constants matching the desktop client configuration
const (
	SIPCredentialKey = "vL4oU4jJ8qS3oC4v"
	PhoneNumberKey   = "2has1d8jef49v0ru"
)

// DialpadLoginReq matches LoginParams of the desktop client
type DialpadLoginReq struct {
	Account  string `json:"account"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// DialpadLoginResp matches LoginResult of the desktop client
type DialpadLoginResp struct {
	UserInfo           DialpadUserInfo `json:"userInfo"`
	Token              string          `json:"token"`
	InactivityDuration int             `json:"inactivityDurationSec"`
	WhitelistDomains   string          `json:"whitelistDomains"`
}

type DialpadUserInfo struct {
	ID         int               `json:"id"`
	Username   string            `json:"username"`
	SeatNumber string            `json:"seatNumber"`
	RoleDetail DialpadRoleDetail `json:"roleDetail"`
}

type DialpadRoleDetail struct {
	Permissions []string `json:"permissions"`
}

// ExtensionInfo matches ExtensionInfo of the desktop client
type ExtensionInfo struct {
	Number     string `json:"number"`   // encrypted hex
	Password   string `json:"password"` // encrypted hex
	Domain     string `json:"domain"`
	Port       string `json:"port"`
	Protocol   string `json:"protocol"`
	ICEServers string `json:"iceServers"`
}

// DialpadCallReq matches CallParams of the desktop client
type DialpadCallReq struct {
	CalledNumber string            `json:"calledNumber"` // encrypted hex
	Extra        map[string]string `json:"extra,omitempty"`
}

// CallPageParams matches CallPageParams of the desktop client
type CallPageParams struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// CallPageResult matches CallPageResult of the desktop client
type CallPageResult struct {
	List    []CallRecord `json:"list"`
	Total   int          `json:"total"`
	HasMore bool         `json:"hasMore"`
}

type CallRecord struct {
	ID        int    `json:"id"`
	CalledNum string `json:"calledNumber"`
	Status    string `json:"status"`
	Duration  int    `json:"duration"`
	Location  string `json:"location"`
	CreatedAt string `json:"createdAt"`
}

type CallTotalResult struct {
	TodayTotal        int     `json:"todayTotal"`
	TodayConnected    int     `json:"todayConnected"`
	TodayDisconnected int     `json:"todayDisconnected"`
	MonthTotal        int     `json:"monthTotal"`
	MonthConnected    int     `json:"monthConnected"`
	Over30sToday      int     `json:"over30sToday"`
	Over30sMonth      int     `json:"over30sMonth"`
	Over30sRateToday  float64 `json:"over30sRateToday"`
	Over30sRateMonth  float64 `json:"over30sRateMonth"`
}

// isNewerVersionGo 比较两个版本号，如果 v1 > v2 则返回 true。
func isNewerVersionGo(v1, v2 string) bool {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// 剥离预发布后缀（如 "-beta", "-rc1"）
	v1Base := strings.SplitN(v1, "-", 2)[0]
	v2Base := strings.SplitN(v2, "-", 2)[0]

	v1Parts := strings.Split(v1Base, ".")
	v2Parts := strings.Split(v2Base, ".")

	maxLen := len(v1Parts)
	if len(v2Parts) > maxLen {
		maxLen = len(v2Parts)
	}

	for i := 0; i < maxLen; i++ {
		var val1, val2 int
		if i < len(v1Parts) {
			val1, _ = strconv.Atoi(v1Parts[i])
		}
		if i < len(v2Parts) {
			val2, _ = strconv.Atoi(v2Parts[i])
		}
		if val1 > val2 {
			return true
		}
		if val1 < val2 {
			return false
		}
	}

	v1HasPre := strings.Contains(v1, "-")
	v2HasPre := strings.Contains(v2, "-")
	if v2HasPre && !v1HasPre {
		return true
	}
	return false
}

// versionCheckMiddleware 提供云枢桌面客户端的强制更新校验中间件。
// 拦截所有需要强更验证的客户端 API，当检测到数据库中存在比客户端当前版本更高的强制更新版本时，返回 426 StatusUpgradeRequired。
func versionCheckMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 对非拨号盘商户客户端 API 直接放行
		if !strings.HasPrefix(path, "/mer/") {
			c.Next()
			return
		}

		// 排除版本检查、升级包下载以及版本上传/管理相关的 API 避免死循环
		if strings.Contains(path, "/version/dialpad") ||
			strings.Contains(path, "/version/download") ||
			strings.Contains(path, "/version/upload") ||
			strings.Contains(path, "/version/list") ||
			strings.Contains(path, "/version/delete") {
			c.Next()
			return
		}

		clientVersion := c.GetHeader("yunshu_version")
		if clientVersion == "" {
			c.Next()
			return
		}

		// 查询数据库中所有未删除且设置为强制更新的版本记录
		var versions []resource.DialpadVersionModel
		err := db.Where("force_update = ? AND del_flag = ?", true, false).Find(&versions).Error
		if err == nil {
			for _, v := range versions {
				// 如果该强更版本大于客户端当前版本，则执行拦截
				if isNewerVersionGo(v.Version, clientVersion) {
					slog.Warn("云枢客户端版本过低且存在更高的强制更新版本", 
						"clientVersion", clientVersion, 
						"requiredVersion", v.Version)
					c.JSON(http.StatusUpgradeRequired, contracts.Fail(426, "云枢客户端当前版本过低，请更新至最新版本以继续使用"))
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// RegisterDialpadCompatRoutes registers the old Dialpad "/mer" API compatibility layer routes
func RegisterDialpadCompatRoutes(
	r gin.IRoutes,
	authService *authdomain.AuthService,
	callRecordService *operatedomain.CallRecordManagementService,
	extensionService *operatedomain.ExtensionManagementService,
	db *gorm.DB,
	updateCfg config.DialpadUpdateConfig,
) {
	// 创建路由子组并装载版本及强制更新校验中间件，以确保客户端升级闭环
	var group gin.IRoutes = r
	if rg, ok := r.(*gin.Engine); ok {
		group = rg.Group("/")
	} else if rg, ok := r.(*gin.RouterGroup); ok {
		group = rg.Group("/")
	}
	group.Use(versionCheckMiddleware(db))
	r = group

	// 1. Dialpad Login
	r.POST("/mer/auth/dialpad/login", func(c *gin.Context) {
		var req DialpadLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		if req.Account == "" || req.Username == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "商户、用户名或密码不能为空"))
			return
		}

		// Look up merchant by account name
		var mch resource.MerchantModel
		if err := db.Where("account = ? AND del_flag = ?", req.Account, false).First(&mch).Error; err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "商户账号不存在"))
			return
		}

		if !mch.Enable {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "该商户账号已被停用"))
			return
		}

		if mch.ExpiredTime != nil && mch.ExpiredTime.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "该商户服务已到期"))
			return
		}

		// Look up user account in cc_sys_account
		var account struct {
			ID           int
			Username     string
			PasswordHash string
			MerchantID   string
			UserID       string
			RoleID       string
			AccountType  string
			DataScope    string
			Enable       bool
		}
		if err := db.Table("cc_sys_account").
			Where("username = ? AND merchant_id = ? AND del_flag = ?", req.Username, strconv.Itoa(mch.ID), false).
			First(&account).Error; err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "用户不存在或不属于该商户"))
			return
		}

		if !account.Enable {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "用户账号已被停用"))
			return
		}

		// Verify password using bcrypt
		if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "用户名或密码错误"))
			return
		}

		// Issue authentication ticket
		ticket, err := authService.Login(c.Request.Context(), authdomain.LoginRequest{
			Username:   req.Username,
			Password:   req.Password,
			MerchantID: strconv.Itoa(mch.ID),
			UserID:     account.UserID,
			RoleID:     account.RoleID,
			DataScope:  account.DataScope,
			Internal:   account.AccountType == operatedomain.AccountTypeSuperAdmin,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "签发登录凭据失败"))
			return
		}

		// Get user's seat number from cc_res_mch_user
		var seatNumber string
		if account.UserID != "" {
			var mchUser resource.MerchantUserModel
			if err := db.Where("id = ? AND del_flag = ?", account.UserID, false).First(&mchUser).Error; err == nil {
				seatNumber = mchUser.SeatNumber
			}
		}

		// --- 自动分配分机 ---
		// 登录成功后，如果该坐席还没有绑定分机，自动从商户的空闲分机池中分配一个。
		// 一个分机同一时间只能绑定一个坐席；坐席退出后分机会被释放回池中。
		uid, _ := strconv.Atoi(account.UserID)
		if uid > 0 {
			// 检查是否已经有绑定的分机
			var existingCount int64
			db.Table("cc_res_extension").
				Where("user_id = ? AND merchant_id = ? AND del_flag = ? AND enable = ?", uid, mch.ID, false, true).
				Count(&existingCount)

			if existingCount == 0 {
				// 没有已绑定分机 → 自动分配一个空闲分机
				var freeExt resource.ExtensionModel
				allocErr := db.Transaction(func(tx *gorm.DB) error {
					// 使用 FOR UPDATE 防止并发分配同一个分机
					err := tx.Where("merchant_id = ? AND user_id = 0 AND enable = ? AND del_flag = ?", mch.ID, true, false).
						Order("id ASC").
						First(&freeExt).Error
					if err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							return errors.New("当前没有可用的空闲分机，请联系管理员分配")
						}
						return err
					}
					// 绑定到当前坐席
					return tx.Model(&resource.ExtensionModel{}).
						Where("id = ?", freeExt.ID).
						Updates(map[string]any{
							"user_id":      uid,
							"bind_type":    2, // 动态绑定（自动回收）
							"offline_at":   nil,
							"updated_time": time.Now().UTC(),
						}).Error
				})
				if allocErr != nil {
					// 自动分配失败，回滚 auth ticket 并拒绝登录
					_ = authService.Logout(c.Request.Context(), ticket.Token)
					c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, allocErr.Error()))
					return
				}
			}
		}

		// Add default permissions to make frontend dials work
		permissions := ticket.Tenant.Permissions
		if len(permissions) == 0 {
			permissions = []string{"dial-pad:direct-call", "dial-pad:record-view"}
		} else {
			// Append these standard permissions if not present
			hasDirectCall := false
			hasRecordView := false
			for _, p := range permissions {
				if p == "dial-pad:direct-call" {
					hasDirectCall = true
				}
				if p == "dial-pad:record-view" {
					hasRecordView = true
				}
			}
			if !hasDirectCall {
				permissions = append(permissions, "dial-pad:direct-call")
			}
			if !hasRecordView {
				permissions = append(permissions, "dial-pad:record-view")
			}
		}

		c.JSON(http.StatusOK, contracts.OK(DialpadLoginResp{
			UserInfo: DialpadUserInfo{
				ID:         account.ID,
				Username:   account.Username,
				SeatNumber: seatNumber,
				RoleDetail: DialpadRoleDetail{
					Permissions: permissions,
				},
			},
			Token:              ticket.Token,
			InactivityDuration: 300,
			WhitelistDomains:   mch.WhitelistDomains,
		}))
	})

	// 2. Dialpad Logout — 同时释放坐席绑定的分机
	r.POST("/mer/auth/dialpad/logout", func(c *gin.Context) {
		tenant, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}

		// 释放该坐席绑定的分机
		releaseExtensionForUser(db, tenant)

		token := tokenFromRequest(c)
		_ = authService.Logout(c.Request.Context(), token)
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"logout": true}))
	})

	// 2b. Release Extension — 仅释放分机（用于客户端关闭时调用）
	r.POST("/mer/v1/user/dialpad/releaseExtension", func(c *gin.Context) {
		tenant, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}
		releaseExtensionForUser(db, tenant)
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"released": true}))
	})

	// 3. Get Extension Info (Protected)
	r.GET("/mer/v1/user/dialpad/extensionInfo", func(c *gin.Context) {
		tenant, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}

		userID, err := strconv.Atoi(tenant.UserID)
		if err != nil || userID <= 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "坐席 ID 无效"))
			return
		}

		var ext resource.ExtensionModel
		if err := db.Where("user_id = ? AND del_flag = ? AND enable = ?", userID, false, true).
			Order("updated_time DESC, id DESC").First(&ext).Error; err != nil {
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "未给此坐席分配或启用分机"))
			return
		}

		var mch resource.MerchantModel
		sipDomain := "127.0.0.1"
		if err := db.Where("id = ? AND del_flag = ?", ext.MerchantID, false).First(&mch).Error; err == nil && mch.SipDomain != "" {
			sipDomain = mch.SipDomain
		}

		// Encrypt credentials
		encryptedNum, err := encryptECBHex(ext.ExtensionNumber, []byte(SIPCredentialKey))
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "分机加密错误"))
			return
		}

		encryptedPassword, err := encryptECBHex(ext.Password, []byte(SIPCredentialKey))
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "密码加密错误"))
			return
		}

		c.JSON(http.StatusOK, contracts.OK(ExtensionInfo{
			Number:     encryptedNum,
			Password:   encryptedPassword,
			Domain:     sipDomain,
			Port:       "5060",
			Protocol:   "udp",
			ICEServers: "",
		}))
	})

	// 4. Valid Number check
	r.GET("/mer/v1/user/dialpad/checkIfUserHasValidNumber", func(c *gin.Context) {
		_, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, contracts.OK(true))
	})

	// 5. Make Call (REST audit/sync)
	r.POST("/mer/v1/call", func(c *gin.Context) {
		_, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}

		var req DialpadCallReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		if req.CalledNumber == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "被叫号码不能为空"))
			return
		}

		// Decrypt number to make sure it's valid
		_, err := decryptECBHex(req.CalledNumber, []byte(PhoneNumberKey))
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "号码解密失败"))
			return
		}

		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	// 6. Paginated Call History
	r.POST("/mer/v1/record/dialpad/call-page", func(c *gin.Context) {
		tenant, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}

		var params CallPageParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		userID, _ := strconv.Atoi(tenant.UserID)
		merchantID, _ := strconv.Atoi(tenant.MerchantID)

		page, err := callRecordService.Page(c.Request.Context(), operatedomain.CallRecordPageRequest{
			PageNumber: params.Page,
			PageSize:   params.Limit,
			MerchantID: merchantID,
			UserID:     userID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询通话记录失败"))
			return
		}

		list := make([]CallRecord, 0, len(page.Records))
		for _, r := range page.Records {
			status := "NO_ANSWER"
			if r.DurationSec > 0 {
				status = "ANSWERED"
			} else if r.HangupCause == "BUSY" || r.HangupCause == "USER_BUSY" {
				status = "BUSY"
			}

			// We need a unique integer ID. Use our stringToID helper
			recordID := stringToID(r.CallID)

			list = append(list, CallRecord{
				ID:        recordID,
				CalledNum: r.Callee,
				Status:    status,
				Duration:  r.DurationSec,
				Location:  "未知",
				CreatedAt: r.CompletedAt.Format("2006-01-02 15:04:05"),
			})
		}

		c.JSON(http.StatusOK, contracts.OK(CallPageResult{
			List:    list,
			Total:   int(page.Total),
			HasMore: int64(params.Page*params.Limit) < page.Total,
		}))
	})

	// 7. Call Statistics Summary
	r.GET("/mer/v1/record/dialpad/call-total", func(c *gin.Context) {
		tenant, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}

		userID, _ := strconv.Atoi(tenant.UserID)

		// Set today and month starting range
		now := time.Now().Local()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

		var todayTotal, todayConnected, monthTotal, monthConnected int64
		var over30sToday, over30sMonth int64

		// Query database records (RecordModel table: cc_biz_cdr)
		db.Model(&business.RecordModel{}).Where("user_id = ? AND completed_at >= ?", userID, todayStart).Count(&todayTotal)
		db.Model(&business.RecordModel{}).Where("user_id = ? AND completed_at >= ? AND duration_sec > 0", userID, todayStart).Count(&todayConnected)
		db.Model(&business.RecordModel{}).Where("user_id = ? AND completed_at >= ? AND duration_sec >= 30", userID, todayStart).Count(&over30sToday)

		db.Model(&business.RecordModel{}).Where("user_id = ? AND completed_at >= ?", userID, monthStart).Count(&monthTotal)
		db.Model(&business.RecordModel{}).Where("user_id = ? AND completed_at >= ? AND duration_sec > 0", userID, monthStart).Count(&monthConnected)
		db.Model(&business.RecordModel{}).Where("user_id = ? AND completed_at >= ? AND duration_sec >= 30", userID, monthStart).Count(&over30sMonth)

		rateToday := 0.0
		if todayConnected > 0 {
			rateToday = float64(over30sToday) / float64(todayConnected)
		}

		rateMonth := 0.0
		if monthConnected > 0 {
			rateMonth = float64(over30sMonth) / float64(monthConnected)
		}

		c.JSON(http.StatusOK, contracts.OK(CallTotalResult{
			TodayTotal:        int(todayTotal),
			TodayConnected:    int(todayConnected),
			TodayDisconnected: int(todayTotal - todayConnected),
			MonthTotal:        int(monthTotal),
			MonthConnected:    int(monthConnected),
			Over30sToday:      int(over30sToday),
			Over30sMonth:      int(over30sMonth),
			Over30sRateToday:  rateToday,
			Over30sRateMonth:  rateMonth,
		}))
	})

	// 8. Auto-Call task stubs
	r.GET("/mer/v1/batch-call-dialpad/ws-pause-status", func(c *gin.Context) {
		_, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{
			"hasTask": false,
			"taskId":  "",
		}))
	})

	r.POST("/mer/v1/batch-call-dialpad/ws-pause-mark", func(c *gin.Context) {
		_, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/mer/v1/batch-call-dialpad/apply-ws-pause", func(c *gin.Context) {
		_, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/mer/v1/batch-call-dialpad/start-task", func(c *gin.Context) {
		_, ok := authenticateDialpad(c, authService)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	// 9. Get Version (GET /mer/version/dialpad)
	r.GET("/mer/version/dialpad", func(c *gin.Context) {
		var latest resource.DialpadVersionModel
		err := db.Where("del_flag = ?", false).Order("id DESC").First(&latest).Error
		if err == nil && latest.Version != "" {
			// 检测客户端传入的版本，如果客户端当前版本比数据库中任何一个强制更新版本旧，则强制将 forceUpdate 设为 true
			clientVersion := c.GetHeader("yunshu_version")
			forceUpdate := latest.ForceUpdate
			if clientVersion != "" {
				var forceVersions []resource.DialpadVersionModel
				db.Where("force_update = ? AND del_flag = ?", true, false).Find(&forceVersions)
				for _, v := range forceVersions {
					if isNewerVersionGo(v.Version, clientVersion) {
						forceUpdate = true
						break
					}
				}
			}

			c.JSON(http.StatusOK, contracts.OK(map[string]any{
				"version":     latest.Version,
				"forceUpdate": forceUpdate,
				"changelog":   latest.Changelog,
			}))
			return
		}

		latestVersion := updateCfg.Version
		if latestVersion == "" {
			latestVersion = "1.0.0"
		}
		changelog := updateCfg.Changelog
		if changelog == "" {
			changelog = "New version available: " + latestVersion
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{
			"version":     latestVersion,
			"forceUpdate": updateCfg.ForceUpdate,
			"changelog":   changelog,
		}))
	})

	// 9b. Download Dialpad Update (GET /mer/version/download/:version/:platform/:arch)
	r.GET("/mer/version/download/:version/:platform/:arch", func(c *gin.Context) {
		version := c.Param("version")
		platform := c.Param("platform")
		arch := c.Param("arch")

		slog.Info("收到拨号盘下载请求", "version", version, "platform", platform, "arch", arch)

		// 优先从数据库查找该特定平台的版本发布包
		var release resource.DialpadVersionModel
		err := db.Where("version = ? AND platform = ? AND arch = ? AND del_flag = ?", version, platform, arch, false).
			Order("id DESC").First(&release).Error
		if err == nil && release.DownloadURL != "" {
			// 如果是 RustFS 外部 S3 直传地址，直接重定向分流下载
			if strings.HasPrefix(release.DownloadURL, "http://") || strings.HasPrefix(release.DownloadURL, "https://") {
				c.Redirect(http.StatusFound, release.DownloadURL)
				return
			}
			// 如果是本地物理路径，直接流式下载
			localPath := release.DownloadURL
			if strings.HasPrefix(localPath, "/") {
				localPath = localPath[1:]
			}
			if _, err := os.Stat(localPath); err == nil {
				c.Header("Content-Description", "File Transfer")
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=yunshu-phone-%s-%s-%s%s", version, platform, arch, filepath.Ext(localPath)))
				c.Header("Content-Type", "application/octet-stream")
				c.File(localPath)
				return
			}
		}

		// 查找编译后的包，本地开发环境我们直接在 sibling folder 或绝对路径下查找各种可能的包格式
		var candidates []string
		if platform == "windows" {
			candidates = []string{
				"../yunshu-phone/build/bin/云枢-amd64-installer.exe",
				"../yunshu-phone/build/bin/云枢.exe",
				"../yunshu-phone/build/bin/yunshu-phone.exe",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢-amd64-installer.exe",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢.exe",
			}
		} else if platform == "darwin" {
			candidates = []string{
				"../yunshu-phone/build/bin/云枢.dmg",
				"../yunshu-phone/build/bin/云枢.pkg",
				"../yunshu-phone/build/bin/云枢.app/Contents/MacOS/云枢",
				"../yunshu-phone/build/bin/yunshu-phone.app/Contents/MacOS/yunshu-phone",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢.dmg",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢.pkg",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢.app/Contents/MacOS/云枢",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/yunshu-phone.app/Contents/MacOS/yunshu-phone",
			}
		} else if platform == "linux" {
			candidates = []string{
				"../yunshu-phone/build/bin/云枢-x86_64.AppImage",
				"../yunshu-phone/build/bin/云枢",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢-x86_64.AppImage",
				"/Users/tangyu/Projects/yunshu-phone/build/bin/云枢",
			}
		}

		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				filename := filepath.Base(path)
				c.Header("Content-Description", "File Transfer")
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
				c.Header("Content-Type", "application/octet-stream")
				c.File(path)
				return
			}
		}

		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, fmt.Sprintf("无法找到该平台的构建包: %s/%s", platform, arch)))
	})

	// 9c. Dialpad Version Management — Upload (POST /mer/version/upload)
	r.POST("/mer/version/upload", func(c *gin.Context) {
		version := c.PostForm("version")
		platform := c.PostForm("platform")
		arch := c.PostForm("arch")
		forceUpdateStr := c.PostForm("forceUpdate")
		changelog := c.PostForm("changelog")

		if version == "" || platform == "" || arch == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数 version, platform, arch 不能为空"))
			return
		}

		// 验证版本号是否符合三段式语义化版本 (SemVer) 格式
		reg := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)
		if !reg.MatchString(version) {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "版本号格式错误，必须为标准的语义化版本号(例如 1.0.0)"))
			return
		}

		// 检查在同一运行平台与架构下，是否已经发布过同版本号的安装包
		var count int64
		db.Model(&resource.DialpadVersionModel{}).
			Where("version = ? AND platform = ? AND arch = ? AND del_flag = ?", version, platform, arch, false).
			Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, fmt.Sprintf("该运行平台及架构下的 v%s 版本已存在，请先在列表中下架原版本包后再重新发布", version)))
			return
		}

		forceUpdate := forceUpdateStr == "true"

		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "未能读取上传的二进制文件"))
			return
		}

		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "打开文件失败"))
			return
		}
		defer src.Close()

		ext := filepath.Ext(file.Filename)
		if ext == "" {
			if platform == "windows" {
				ext = ".exe"
			} else if platform == "darwin" {
				ext = ".app"
			}
		}
		filename := fmt.Sprintf("yunshu-phone-%s-%s-%s%s", version, platform, arch, ext)

		// 实例化 RustFS 存储管理器
		storeCfg := storage.RustFSConfig{
			Endpoint:  updateCfg.RustFS.Endpoint,
			AccessKey: updateCfg.RustFS.AccessKey,
			SecretKey: updateCfg.RustFS.SecretKey,
			Bucket:    updateCfg.RustFS.Bucket,
		}
		storeDriver := storage.NewRustFSStorage(storeCfg)

		downloadURL, err := storeDriver.Store(c.Request.Context(), filename, src)
		if err != nil {
			slog.Error("保存版本升级包失败", "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存升级包失败: "+err.Error()))
			return
		}

		// 落库记录
		versionRecord := resource.DialpadVersionModel{
			Version:     version,
			Platform:    platform,
			Arch:        arch,
			ForceUpdate: forceUpdate,
			Changelog:   changelog,
			FileKey:     filename,
			FileSize:    file.Size,
			DownloadURL: downloadURL,
		}

		if err := db.Create(&versionRecord).Error; err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "写入版本历史库失败"))
			return
		}

		slog.Info("拨号盘版本上传成功", "version", version, "platform", platform, "url", downloadURL)
		c.JSON(http.StatusOK, contracts.OK(map[string]any{
			"id":          versionRecord.ID,
			"downloadUrl": downloadURL,
		}))
	})

	// 9d. Dialpad Version Management — List (GET /mer/version/list)
	r.GET("/mer/version/list", func(c *gin.Context) {
		var list []resource.DialpadVersionModel
		if err := db.Where("del_flag = ?", false).Order("id DESC").Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询版本列表失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	// 9e. Dialpad Version Management — Delete (POST /mer/version/delete)
	r.POST("/mer/version/delete", func(c *gin.Context) {
		var req struct {
			ID int `json:"id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "ID 无效"))
			return
		}

		var record resource.DialpadVersionModel
		if err := db.Where("id = ? AND del_flag = ?", req.ID, false).First(&record).Error; err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "找不到对应的版本发布包"))
			return
		}

		// 实例化存储管理器进行物理包删除（垃圾回收）
		storeCfg := storage.RustFSConfig{
			Endpoint:  updateCfg.RustFS.Endpoint,
			AccessKey: updateCfg.RustFS.AccessKey,
			SecretKey: updateCfg.RustFS.SecretKey,
			Bucket:    updateCfg.RustFS.Bucket,
		}
		storeDriver := storage.NewRustFSStorage(storeCfg)
		if err := storeDriver.Delete(c.Request.Context(), record.FileKey); err != nil {
			slog.Error("下架版本物理文件删除失败，进行软标记处理", "error", err.Error(), "fileKey", record.FileKey)
		}

		if err := db.Model(&resource.DialpadVersionModel{}).Where("id = ?", req.ID).Update("del_flag", true).Error; err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(true))
	})
}

// --- Authenticate / Session Helpers ---

func authenticateDialpad(c *gin.Context, service *authdomain.AuthService) (contracts.TenantContext, bool) {
	token := tokenFromRequest(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "请先登录"))
		return contracts.TenantContext{}, false
	}
	ticket, ok := service.Token(c.Request.Context(), token)
	if !ok {
		c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "token 无效或已过期"))
		return contracts.TenantContext{}, false
	}
	return ticket.Tenant, true
}

func stringToID(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = 31*h + int(s[i])
	}
	if h < 0 {
		h = -h
	}
	return h
}

// --- Extension Release Helper ---

// releaseExtensionForUser 释放指定坐席绑定的所有动态分机。
// 将 user_id 设为 0，bind_type 恢复为 1，offline_at 清除。
func releaseExtensionForUser(db *gorm.DB, tenant contracts.TenantContext) {
	userID, _ := strconv.Atoi(tenant.UserID)
	if userID <= 0 {
		return
	}
	merchantID, _ := strconv.Atoi(tenant.MerchantID)
	now := time.Now().UTC()
	result := db.Model(&resource.ExtensionModel{}).
		Where("user_id = ? AND del_flag = ?", userID, false).
		Where("bind_type = ?", 2). // 只释放动态绑定的分机
		Updates(map[string]any{
			"user_id":      0,
			"bind_type":    1,
			"offline_at":   nil,
			"updated_time": now,
		})
	if result.Error != nil {
		// 静默处理：释放失败不应阻塞退出流程
		return
	}
	if result.RowsAffected > 0 {
		// 触发 auth 缓存失效，使旧的分机绑定信息立即失效
		_ = merchantID // 保留以备后续按需清理
	}
}

// --- AES ECB Padding Helpers ---

func encryptECBHex(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	data := pkcs7Pad([]byte(plaintext), block.BlockSize())
	ciphertext := make([]byte, len(data))
	for i := 0; i < len(data); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], data[i:i+block.BlockSize()])
	}
	return hex.EncodeToString(ciphertext), nil
}

func decryptECBHex(hexText string, key []byte) (string, error) {
	cipherBytes, err := hex.DecodeString(hexText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(cipherBytes)%block.BlockSize() != 0 {
		return "", errors.New("ciphertext is not a multiple of block size")
	}
	plaintext := make([]byte, len(cipherBytes))
	for i := 0; i < len(cipherBytes); i += block.BlockSize() {
		block.Decrypt(plaintext[i:i+block.BlockSize()], cipherBytes[i:i+block.BlockSize()])
	}
	unpadded, err := pkcs7Unpad(plaintext)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding == 0 {
		return nil, errors.New("invalid padding")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}
