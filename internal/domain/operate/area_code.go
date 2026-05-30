package operate

import "context"

// AreaCode 表示行政区域（省、市、区）的区划代码实体。
type AreaCode struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	ParentCode string `json:"parentCode,omitempty"`
	Level      int    `json:"level"` // 1=省/直辖市, 2=地级市/区县
}

// AreaCodeRepository 定义行政区域数据仓储能力。
type AreaCodeRepository interface {
	ListAll(ctx context.Context) ([]AreaCode, error)
	SaveBatch(ctx context.Context, list []AreaCode) error
}
