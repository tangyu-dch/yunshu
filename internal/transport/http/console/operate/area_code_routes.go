package operate

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterAreaCodeRoutes 注册运营端行政区划管理接口。
func RegisterAreaCodeRoutes(r gin.IRoutes, repo operatedomain.AreaCodeRepository) {
	r.GET("/operate/area-code/list", func(c *gin.Context) {
		if repo == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "行政区域服务不可用"))
			return
		}
		list, err := repo.ListAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取行政区域列表失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})
}
