package operate

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/installer"
)

// RegisterInstallerRoutes 注册系统一键安装与初始化部署引导相关的 HTTP 路由端点。
func RegisterInstallerRoutes(r gin.IRoutes, inst *installer.Installer) {
	checkNotInstalled := func(c *gin.Context) {
		if inst.IsInstalled() {
			c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "系统已完成初始化部署，禁止重复安装"))
			c.Abort()
			return
		}
		c.Next()
	}

	r.GET("/api/install/status", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		status := inst.Precheck(ctx)
		c.JSON(http.StatusOK, contracts.OK(status))
	})

	r.POST("/api/install/setup", checkNotInstalled, func(c *gin.Context) {
		var params installer.SetupParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "安装参数解析失败: "+err.Error()))
			return
		}

		// 执行配置文件与 docker-compose.yml 的动态生成写入
		if err := inst.GenerateConfigs(params); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "生成配置文件失败: "+err.Error()))
			return
		}

		c.JSON(http.StatusOK, contracts.OK("基础配置文件生成成功"))
	})

	r.POST("/api/install/deploy", checkNotInstalled, func(c *gin.Context) {
		if err := inst.StartDeployment(); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK("一键容器部署已成功触发，正在进行后台拉起"))
	})

	r.GET("/api/install/deploy/status", func(c *gin.Context) {
		status := inst.DeployStatus()
		c.JSON(http.StatusOK, contracts.OK(status))
	})

	r.POST("/api/install/services/start", checkNotInstalled, func(c *gin.Context) {
		var params installer.SetupParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数解析失败"))
			return
		}

		// 异步检查并初始化数据库和种子填充
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := inst.InitializeDatabase(ctx, params); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "数据库迁移与种子初始化失败: "+err.Error()))
			return
		}

		c.JSON(http.StatusOK, contracts.OK("系统核心数据库结构迁移与种子填充已顺利完成"))
	})
}
