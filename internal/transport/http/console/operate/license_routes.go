package operate

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterLicenseRoutes 注册私有化部署授权管理的运营端 HTTP 接口。
func RegisterLicenseRoutes(r gin.IRoutes, service *operatedomain.LicenseService) {
	if service == nil {
		return
	}

	// 1. 获取本地设备指纹及部署ID
	r.GET("/operate/license/fingerprint", func(c *gin.Context) {
		fp, err := service.GetHardwareFingerprint()
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取硬件指纹失败: "+err.Error()))
			return
		}
		depID, err := service.GetDeploymentID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取部署ID失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(gin.H{
			"deploymentId": depID,
			"uuid":         fp.UUID,
			"macs":         fp.MACs,
			"diskSerial":   fp.DiskSerial,
			"hostname":     fp.Hostname,
		}))
	})

	// 2. 支持下载设备指纹注册信息 (yunshu_register_info.json)
	r.GET("/operate/license/fingerprint/download", func(c *gin.Context) {
		fp, err := service.GetHardwareFingerprint()
		if err != nil {
			c.String(http.StatusInternalServerError, "获取硬件指纹失败: %v", err)
			return
		}
		depID, err := service.GetDeploymentID()
		if err != nil {
			c.String(http.StatusInternalServerError, "获取部署ID失败: %v", err)
			return
		}
		data := map[string]any{
			"deploymentId": depID,
			"uuid":         fp.UUID,
			"macs":         fp.MACs,
			"diskSerial":   fp.DiskSerial,
			"hostname":     fp.Hostname,
			"generatedAt":  time.Now().Format("2006-01-02 15:04:05"),
		}
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			c.String(http.StatusInternalServerError, "序列化硬件信息失败")
			return
		}

		c.Header("Content-Disposition", "attachment; filename=yunshu_register_info.json")
		c.Data(http.StatusOK, "application/json", jsonBytes)
	})

	// 3. 查询授权证书激活及状态信息
	r.GET("/operate/license/status", func(c *gin.Context) {
		status, err := service.GetLicenseStatus(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取授权状态异常: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(status))
	})

	// 4. 上传并激活证书
	r.POST("/operate/license/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "上传文件解析失败: "+err.Error()))
			return
		}
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取上传数据流失败"))
			return
		}
		defer src.Close()

		licBytes, err := io.ReadAll(src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取证书内容失败"))
			return
		}

		err = service.SaveLicense(c.Request.Context(), licBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "证书激活校验失败: "+err.Error()))
			return
		}

		c.JSON(http.StatusOK, contracts.OK("授权激活成功"))
	})

	// 5. 支持下载备份当前正在生效的证书 (yunshu.lic)
	r.GET("/operate/license/download", func(c *gin.Context) {
		data, err := os.ReadFile(service.LicensePath)
		if err != nil {
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "当前系统内未找到已激活的证书文件，请先上传激活"))
			return
		}
		filename := filepath.Base(service.LicensePath)
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Data(http.StatusOK, "application/octet-stream", data)
	})

	// 6. 设置租户模式
	r.POST("/operate/license/tenant-mode", func(c *gin.Context) {
		var req struct {
			Mode string `json:"mode" binding:"required,oneof=single multi"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "无效的租户模式值，仅支持 single 或 multi"))
			return
		}
		if service.Repo == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "仓储服务未就绪"))
			return
		}
		if err := service.Repo.Set(c.Request.Context(), "tenant.mode", req.Mode, "云枢隔离部署模式(single/multi)"); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新租户模式配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK("租户模式更新成功"))
	})
}
