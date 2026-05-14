package gamecode

const (

	// MdNull 没有 md
	MdNull int = 4109

	// SessionNull 没有 session
	SessionNull int = 4110

	// SessionExpire Session 过期
	SessionExpire int = 4111

	// SignAccountEmpty 账号为空。
	SignAccountEmpty int = 10001

	// SignPasswordEmpty 密码为空。
	SignPasswordEmpty int = 10002

	// SignAccountExists 账号已存在。
	SignAccountExists int = 10003

	// SignQueryAccountFailed 查询账号失败。
	SignQueryAccountFailed int = 10004

	// SignCreateUserFailed 创建用户失败。
	SignCreateUserFailed int = 10005

	// SignAccountNotFound 账号不存在。
	SignAccountNotFound int = 10006

	// SignPasswordWrong 密码错误。
	SignPasswordWrong int = 10007

	// SignCreateTokenFailed 创建 token 失败。
	SignCreateTokenFailed int = 10008

	// SignTokenEmpty token 为空。
	SignTokenEmpty int = 10009

	// SignTokenInvalid token 无效或已过期。
	SignTokenInvalid int = 10010

	// SignRefreshTokenFailed 刷新 token 失败。
	SignRefreshTokenFailed int = 10011

	// SignLogoutFailed 退出登录失败。
	SignLogoutFailed int = 10012
)
