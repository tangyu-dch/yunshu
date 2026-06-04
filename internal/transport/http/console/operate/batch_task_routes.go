package operate

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterBatchTaskRoutes 注册商户批量外呼任务管理接口。
func RegisterBatchTaskRoutes(r gin.IRoutes, service *operatedomain.BatchTaskManagementService) {
	r.POST("/merchant/batch-call-task/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		var req operatedomain.BatchTaskPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		// 后端数据隔离校验：限制非运营账户只能查自身商户的数据
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok {
			if !tenant.Internal {
				parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
				req.MerchantID = parsedMerchantID
			} else {
				if req.MerchantID <= 0 {
					if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
						if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
							req.MerchantID = parsed
						}
					}
				}
			}
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询批量任务失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/batch-call-task/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		task, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeBatchTaskError(c, err, "批量任务不存在")
			return
		}
		// 后端数据隔离校验：限制非运营账户只能查自身商户的数据
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			if task.MerchantID != parsedMerchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权访问此任务详情"))
				return
			}
		}
		c.JSON(http.StatusOK, contracts.OK(task))
	})

	r.PUT("/merchant/batch-call-task/add", saveBatchTask(service))
	r.POST("/merchant/batch-call-task/update", saveBatchTask(service))
	r.POST("/merchant/batch-call-task/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		ids, ok := parseIDs(c)
		if !ok {
			return
		}
		// 后端数据隔离校验：非运营账户只能删除自身商户的任务
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			for _, id := range ids {
				task, err := service.Repository.GetByID(c.Request.Context(), id)
				if err == nil && task.MerchantID != parsedMerchantID {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权删除其他商户的任务"))
					return
				}
			}
		}
		if err := service.Delete(c.Request.Context(), ids); err != nil {
			writeBatchTaskError(c, err, "删除批量任务失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(ids)}))
	})

	r.POST("/merchant/batch-call-task/enable/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		// 后端数据隔离校验
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			task, err := service.Repository.GetByID(c.Request.Context(), id)
			if err == nil && task.MerchantID != parsedMerchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权启用其他商户的任务"))
				return
			}
		}
		task, err := service.SetEnable(c.Request.Context(), id, true, "")
		if err != nil {
			writeBatchTaskError(c, err, "启用批量任务失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(task))
	})

	r.POST("/merchant/batch-call-task/disable/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		// 后端数据隔离校验
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			task, err := service.Repository.GetByID(c.Request.Context(), id)
			if err == nil && task.MerchantID != parsedMerchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权停用其他商户的任务"))
				return
			}
		}
		reason := c.Query("reason")
		if reason == "" {
			reason = "手动停用"
		}
		task, err := service.SetEnable(c.Request.Context(), id, false, reason)
		if err != nil {
			writeBatchTaskError(c, err, "停用批量任务失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(task))
	})

	r.GET("/merchant/batch-call-task", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		merchantID, _ := strconv.Atoi(c.Query("merchantId"))
		userID, _ := strconv.Atoi(c.Query("userId"))
		// 后端数据隔离校验
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok {
			if !tenant.Internal {
				merchantID, _ = strconv.Atoi(tenant.MerchantID)
				userID, _ = strconv.Atoi(tenant.UserID)
			} else {
				if merchantID <= 0 {
					if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
						if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
							merchantID = parsed
						}
					}
				}
			}
		}
		page, err := service.Page(c.Request.Context(), operatedomain.BatchTaskPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			MerchantID: merchantID,
			UserID:     userID,
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询批量任务失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.POST("/merchant/batch-call-task/import/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			MerchantID int      `json:"merchantId"`
			UserID     int      `json:"userId"`
			Tels       []string `json:"tels"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		// 后端数据隔离校验
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth {
			if !tenant.Internal {
				req.MerchantID, _ = strconv.Atoi(tenant.MerchantID)
				req.UserID, _ = strconv.Atoi(tenant.UserID)
			} else {
				if req.MerchantID <= 0 {
					if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
						if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
							req.MerchantID = parsed
						}
					}
				}
			}
			// 验证任务所属商户符合请求者身份
			task, err := service.Repository.GetByID(c.Request.Context(), id)
			if err == nil && task.MerchantID != req.MerchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权为此任务导入数据"))
				return
			}
		}
		if err := service.ImportTels(c.Request.Context(), id, req.MerchantID, req.UserID, req.Tels); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "导入号码失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"imported": len(req.Tels)}))
	})

	r.GET("/merchant/batch-call-task/import/template", func(c *gin.Context) {
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", "attachment; filename=import_template.csv")
		c.Header("Content-Type", "text/csv; charset=utf-8")

		// 写入 UTF-8 BOM 字节集，防止 Excel 打开中文乱码
		_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
		c.String(http.StatusOK, "手机号码,客户姓名,备注\n13800000001,张三,示例备注\n13800000002,李四,示例备注\n")
	})

	r.POST("/merchant/batch-call-task/import/file/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}

		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "未检测到上传的文件"))
			return
		}

		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "无法打开上传的文件"))
			return
		}
		defer src.Close()

		// 解析 CSV
		reader := csv.NewReader(src)
		reader.FieldsPerRecord = -1

		records, err := reader.ReadAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "解析 CSV 文件失败，请确保格式正确"))
			return
		}

		var tels []string
		for i, record := range records {
			if i == 0 {
				// 跳过首行表头
				continue
			}
			if len(record) > 0 {
				tel := strings.TrimSpace(record[0])
				// 提取纯数字的电话号码 (基本验证，如长度限制，剔除非数字)
				var cleanTel strings.Builder
				for _, r := range tel {
					if r >= '0' && r <= '9' {
						cleanTel.WriteRune(r)
					}
				}
				finalTel := cleanTel.String()
				if finalTel != "" {
					tels = append(tels, finalTel)
				}
			}
		}

		var merchantID int
		var userID int
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth {
			if !tenant.Internal {
				merchantID, _ = strconv.Atoi(tenant.MerchantID)
				userID, _ = strconv.Atoi(tenant.UserID)
			} else {
				if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
					if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
						merchantID = parsed
					}
				}
				if ctxUserID := c.GetHeader("X-User-Id"); ctxUserID != "" {
					if parsed, err := strconv.Atoi(ctxUserID); err == nil {
						userID = parsed
					}
				}
			}
			// 验证任务所属商户符合请求者身份
			task, err := service.Repository.GetByID(c.Request.Context(), id)
			if err == nil && task.MerchantID != merchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权为此任务导入数据"))
				return
			}
		}

		if err := service.ImportTels(c.Request.Context(), id, merchantID, userID, tels); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "导入号码失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"imported": len(tels)}))
	})

	r.GET("/merchant/batch-call-task/details/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		// 后端数据隔离校验
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			task, err := service.Repository.GetByID(c.Request.Context(), id)
			if err == nil && task.MerchantID != parsedMerchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权访问此任务拨打明细"))
				return
			}
		}
		details, err := service.GetDetails(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询拨打明细失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(details))
	})
}

// RegisterBatchDialpadRoutes 注册商户批量外呼 dialpad 控制接口。
func RegisterBatchDialpadRoutes(r gin.IRoutes, service *operatedomain.BatchTaskManagementService) {
	r.GET("/merchant/batch-call-dialpad/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		task, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeBatchTaskError(c, err, "dialpad 任务不存在")
			return
		}
		// 后端数据隔离校验
		tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
		if okAuth && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			if task.MerchantID != parsedMerchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权访问此拨号盘详情"))
				return
			}
		}
		c.JSON(http.StatusOK, contracts.OK(task))
	})

	r.POST("/merchant/batch-call-dialpad/start/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		toggleDialpad(c, service, true, "")
	})

	r.POST("/merchant/batch-call-dialpad/pause/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		reason := c.Query("reason")
		if reason == "" {
			reason = "手动暂停"
		}
		toggleDialpad(c, service, false, reason)
	})

	r.POST("/merchant/batch-call-dialpad/resume/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		toggleDialpad(c, service, true, "")
	})

	r.POST("/merchant/batch-call-dialpad/disconnect-pause/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量任务管理未启用"))
			return
		}
		reason := c.Query("reason")
		if reason == "" {
			reason = "线路断开"
		}
		toggleDialpad(c, service, false, reason)
	})
}

func saveBatchTask(service *operatedomain.BatchTaskManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operatedomain.BatchTask
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		// 后端数据隔离校验：限制非运营账户只能保存自身商户的数据
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok {
			if !tenant.Internal {
				parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
				req.MerchantID = parsedMerchantID
				parsedUserID, _ := strconv.Atoi(tenant.UserID)
				req.UserID = parsedUserID
			} else {
				if req.MerchantID <= 0 {
					if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
						if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
							req.MerchantID = parsed
						}
					}
				}
			}
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeBatchTaskError(c, err, "保存批量任务失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func toggleDialpad(c *gin.Context, service *operatedomain.BatchTaskManagementService, enable bool, reason string) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	// 后端数据隔离校验
	tenant, okAuth := contracts.TenantFromContext(c.Request.Context())
	if okAuth && !tenant.Internal {
		parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
		task, err := service.Repository.GetByID(c.Request.Context(), id)
		if err == nil && task.MerchantID != parsedMerchantID {
			c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权控制其他商户的拨号盘"))
			return
		}
	}
	task, err := service.SetEnable(c.Request.Context(), id, enable, reason)
	if err != nil {
		writeBatchTaskError(c, err, "批量任务状态切换失败")
		return
	}
	c.JSON(http.StatusOK, contracts.OK(map[string]any{"task": task, "refreshRequired": true}))
}

func writeBatchTaskError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, operatedomain.ErrBatchTaskNotFound) {
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "批量任务不存在"))
		return
	}
	if errors.Is(err, operatedomain.ErrInvalidBatchTask) {
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "批量任务参数错误"))
		return
	}
	c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
}

func parseIDs(c *gin.Context) ([]int, bool) {
	var req []struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
		return nil, false
	}
	ids := make([]int, 0, len(req))
	for _, item := range req {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "任务 ID 错误"))
		return nil, false
	}
	return ids, true
}
