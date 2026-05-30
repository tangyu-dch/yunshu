// contracts 包定义了呼叫中心系统的对外契约，包括 HTTP API、Redis、MQ、错误码和共享类型。
// 所有跨服务通信都必须遵循本包定义的接口和数据结构，任何修改都是兼容性契约变更。
package contracts

import (
	"encoding/json"
	"net/http"
)

// 业务错误码常量，兼容  Yunshu CallCenter 接口契约。
// 4600 以下为 HTTP 标准错误码，4600 及以上为业务自定义错误码。
const (
	CodeOK                  = 0    // 成功
	CodeBadRequest          = 400  // 请求参数错误，客户端需修正请求
	CodeUnauthorized        = 401  // 未登录或签名无效，需重新认证
	CodeForbidden           = 403  // 无权限访问，客户端无此操作权限
	CodeNotFound            = 404  // 资源不存在
	CodeConflict            = 409  // 状态冲突，可能因并发操作导致
	CodeInternal            = 500  // 系统内部错误，需查看日志定位问题
	CodeSelectionFailed     = 4601 // 选号失败，CTI 无法为当前请求分配可用号码
	CodeDuplicateCommand    = 4602 // 重复命令，幂等键命中，需检查是否已处理过
	CodeResourceUnavailable = 4603 // 资源不可用，如分机忙、网关离线等
)

// Result 是 HTTP API 的标准响应结构。
// Code 为 0 表示成功，非零表示业务错误；Message 是中文错误描述；Data 承载成功时的业务数据。
type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 返回成功响应，数据载荷通过 data 参数传入。
// 使用此函数可确保 Code 始终为 0，Message 为"成功"。
func OK(data any) Result {
	return Result{Code: CodeOK, Message: "成功", Data: data}
}

// Fail 返回失败响应，code 必须为非零错误码。
// message 应使用中文描述具体失败原因，便于调用方和运维人员理解。
func Fail(code int, message string) Result {
	return Result{Code: code, Message: message}
}

// WriteJSON 将 Result 写入 HTTP 响应，自动设置 Content-Type 为 application/json。
// status 参数决定 HTTP 状态码，200 用于业务错误（如选号失败），实际错误码在 Result.Code 中。
func WriteJSON(w http.ResponseWriter, status int, result Result) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}
