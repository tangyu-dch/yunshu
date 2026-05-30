// contracts 包定义了呼叫中心系统的对外契约，包括 HTTP API、Redis、MQ、错误码和共享类型。
// 所有跨服务通信都必须遵循本包定义的接口和数据结构，任何修改都是兼容性契约变更。
package contracts

// ErrorContract 定义了对外可见错误码的完整语义。
// Code 是错误码数字标识，Key 是错误标识符，Message 是中文错误描述，
// HTTPStatus 指示对应的 HTTP 状态码，Retryable 表示该错误是否可重试，
// Owner 标识该错误的负责领域（platform=平台级，auth=认证授权，cti=CTI业务，esl=ESL业务）。
type ErrorContract struct {
	Code       int    `json:"code"`
	Key        string `json:"key"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"httpStatus"`
	Retryable  bool   `json:"retryable"`
	Owner      string `json:"owner"`
}

// ErrorContracts 记录对外可见错误码。
// 修改这里意味着兼容契约变化，需要同步 contract test、接口文档和调用方预期。
var ErrorContracts = []ErrorContract{
	{Code: CodeOK, Key: "ok", Message: "成功", HTTPStatus: 200, Retryable: false, Owner: "platform"},
	{Code: CodeBadRequest, Key: "bad_request", Message: "请求参数错误", HTTPStatus: 400, Retryable: false, Owner: "platform"},
	{Code: CodeUnauthorized, Key: "unauthorized", Message: "未登录或签名无效", HTTPStatus: 401, Retryable: false, Owner: "auth"},
	{Code: CodeForbidden, Key: "forbidden", Message: "无权限访问", HTTPStatus: 403, Retryable: false, Owner: "auth"},
	{Code: CodeNotFound, Key: "not_found", Message: "资源不存在", HTTPStatus: 404, Retryable: false, Owner: "platform"},
	{Code: CodeConflict, Key: "conflict", Message: "状态冲突", HTTPStatus: 409, Retryable: true, Owner: "platform"},
	{Code: CodeInternal, Key: "internal_error", Message: "系统异常", HTTPStatus: 500, Retryable: true, Owner: "platform"},
	{Code: CodeSelectionFailed, Key: "selection_failed", Message: "选号失败", HTTPStatus: 200, Retryable: true, Owner: "cti"},
	{Code: CodeDuplicateCommand, Key: "duplicate_command", Message: "重复命令", HTTPStatus: 200, Retryable: false, Owner: "cti/esl"},
	{Code: CodeResourceUnavailable, Key: "resource_unavailable", Message: "资源不可用", HTTPStatus: 200, Retryable: true, Owner: "cti/esl"},
}
