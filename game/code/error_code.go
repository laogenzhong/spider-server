package errorgamecode

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

	// SignDeleteAccountFailed 删除账号失败。
	SignDeleteAccountFailed int = 10013

	// SignAppleIdentityTokenEmpty Apple 登录 identityToken 为空。
	SignAppleIdentityTokenEmpty int = 10014

	// SignAppleIdentityTokenInvalid Apple 登录 identityToken 无效。
	SignAppleIdentityTokenInvalid int = 10015

	// SignAppleNonceInvalid Apple 登录 nonce 校验失败。
	SignAppleNonceInvalid int = 10016

	// SignAppleConfigInvalid Apple 登录服务端配置无效。
	SignAppleConfigInvalid int = 10017

	// SignAppleTokenExchangeFailed Apple 登录授权码换取 token 失败。
	SignAppleTokenExchangeFailed int = 10018

	// SignAppleAccountBindFailed Apple 登录账号绑定失败。
	SignAppleAccountBindFailed int = 10019

	// SignAppleTokenRevokeFailed Apple 登录授权撤销失败。
	SignAppleTokenRevokeFailed int = 10020

	// SignRegistrationDisabled 自定义账号注册已关闭。
	SignRegistrationDisabled int = 10021

	// WeightRecordDateEmpty 体重记录日期为空。
	WeightRecordDateEmpty int = 20001

	// WeightValueInvalid 体重值无效。
	WeightValueInvalid int = 20002

	// WeightSatietyInvalid 饱腹感评分无效。
	WeightSatietyInvalid int = 20003

	// WeightSaveFailed 保存体重记录失败。
	WeightSaveFailed int = 20004

	// WeightDeleteKeyEmpty 删除体重记录缺少 id 或日期。
	WeightDeleteKeyEmpty int = 20005

	// WeightDeleteFailed 删除体重记录失败。
	WeightDeleteFailed int = 20006

	// WeightQueryFailed 查询体重记录失败。
	WeightQueryFailed int = 20007

	// WeightDateRangeEmpty 体重记录日期范围为空。
	WeightDateRangeEmpty int = 20008

	// WeightListFailed 查询体重记录列表失败。
	WeightListFailed int = 20009

	// WeightLatestQueryFailed 查询最近体重记录失败。
	WeightLatestQueryFailed int = 20010

	// WeightDailyCreateLimitExceeded 当天体重记录新增数量超过上限。
	WeightDailyCreateLimitExceeded int = 20011

	// TrainingTagNameEmpty 训练标签名称为空。
	TrainingTagNameEmpty int = 30001

	// TrainingTagCreateFailed 创建训练标签失败。
	TrainingTagCreateFailed int = 30002

	// TrainingTagIDEmpty 训练标签 id 为空。
	TrainingTagIDEmpty int = 30003

	// TrainingTagNotFound 训练标签不存在。
	TrainingTagNotFound int = 30004

	// TrainingTagUpdateFailed 修改训练标签失败。
	TrainingTagUpdateFailed int = 30005

	// TrainingTagDeleteFailed 删除训练标签失败。
	TrainingTagDeleteFailed int = 30006

	// TrainingTagListFailed 查询训练标签列表失败。
	TrainingTagListFailed int = 30007

	// TrainingTagReorderFailed 调整训练标签排序失败。
	TrainingTagReorderFailed int = 30008

	// WorkoutTagsTargetEmpty 训练标签绑定缺少 workout_uuid 或训练时间。
	WorkoutTagsTargetEmpty int = 30009

	// WorkoutTagsSaveFailed 保存训练标签绑定失败。
	WorkoutTagsSaveFailed int = 30010

	// WorkoutTagsQueryFailed 查询训练标签绑定失败。
	WorkoutTagsQueryFailed int = 30011

	// WorkoutTagsDeleteFailed 删除训练标签绑定失败。
	WorkoutTagsDeleteFailed int = 30012

	// WorkoutTagsRecordDateEmpty 查询当天训练标签缺少日期。
	WorkoutTagsRecordDateEmpty int = 30013

	// WorkoutTagsDailyQueryFailed 查询当天训练标签失败。
	WorkoutTagsDailyQueryFailed int = 30014

	// WorkoutTagsDateRangeEmpty 查询范围训练标签缺少日期范围。
	WorkoutTagsDateRangeEmpty int = 30015

	// WorkoutTagsRangeQueryFailed 查询范围训练标签失败。
	WorkoutTagsRangeQueryFailed int = 30016

	// TrainingTagLimitExceeded 训练标签数量超过上限。
	TrainingTagLimitExceeded int = 30017

	// TrainingTagDailyLimitExceeded 当天训练标签新增数量超过上限。
	TrainingTagDailyLimitExceeded int = 30018

	// WorkoutLocationTargetEmpty 训练位置缺少 workout_uuid 或训练时间。
	WorkoutLocationTargetEmpty int = 30019

	// WorkoutLocationInvalid 训练位置坐标无效。
	WorkoutLocationInvalid int = 30020

	// WorkoutLocationSaveFailed 保存训练位置失败。
	WorkoutLocationSaveFailed int = 30021

	// WorkoutLocationQueryFailed 查询训练位置失败。
	WorkoutLocationQueryFailed int = 30022

	// WorkoutNoteTargetEmpty 训练备注缺少 workout_uuid 或训练时间。
	WorkoutNoteTargetEmpty int = 30023

	// WorkoutNoteTooLong 训练备注超过长度限制。
	WorkoutNoteTooLong int = 30024

	// WorkoutNoteSaveFailed 保存训练备注失败。
	WorkoutNoteSaveFailed int = 30025

	// WorkoutNoteQueryFailed 查询训练备注失败。
	WorkoutNoteQueryFailed int = 30026

	// RestoreStartSnapshotInvalid 起始快照无效。
	RestoreStartSnapshotInvalid int = 40001

	// RestoreStartAfterCurrent 起始快照大于当前服务端快照。
	RestoreStartAfterCurrent int = 40002

	// RestoreCountWeightFailed 统计体重同步数据失败。
	RestoreCountWeightFailed int = 40003

	// RestoreCountTrainingTagsFailed 统计训练标签同步数据失败。
	RestoreCountTrainingTagsFailed int = 40004

	// RestoreCountWorkoutTagBindingsFailed 统计训练标签绑定同步数据失败。
	RestoreCountWorkoutTagBindingsFailed int = 40005

	// RestoreCountBodyPhotosFailed 统计照片索引同步数据失败。
	RestoreCountBodyPhotosFailed int = 40006

	// RestoreEndSnapshotEmpty 结束快照为空。
	RestoreEndSnapshotEmpty int = 40007

	// RestoreStartAfterEnd 起始快照大于结束快照。
	RestoreStartAfterEnd int = 40008

	// RestoreTaskIDEmpty 同步任务 id 为空。
	RestoreTaskIDEmpty int = 40009

	// RestoreFetchWeightFailed 拉取体重同步数据失败。
	RestoreFetchWeightFailed int = 40010

	// RestoreFetchTrainingTagsFailed 拉取训练标签同步数据失败。
	RestoreFetchTrainingTagsFailed int = 40011

	// RestoreFetchWorkoutTagBindingsFailed 拉取训练标签绑定同步数据失败。
	RestoreFetchWorkoutTagBindingsFailed int = 40012

	// RestoreFetchBodyPhotosFailed 拉取照片索引同步数据失败。
	RestoreFetchBodyPhotosFailed int = 40013

	// RestoreTaskUnknown 未知同步任务。
	RestoreTaskUnknown int = 40014

	// FriendProfileQueryFailed 获取朋友资料失败。
	FriendProfileQueryFailed int = 50001

	// FriendListQueryFailed 获取好友列表失败。
	FriendListQueryFailed int = 50002

	// FriendRemarkQueryFailed 获取好友备注失败。
	FriendRemarkQueryFailed int = 50003

	// FriendUserIDEmpty 好友用户 id 为空。
	FriendUserIDEmpty int = 50004

	// FriendUserNotFound 用户不存在。
	FriendUserNotFound int = 50005

	// FriendRequestSendFailed 发送好友申请失败。
	FriendRequestSendFailed int = 50006

	// FriendRequestListFailed 获取好友申请失败。
	FriendRequestListFailed int = 50007

	// FriendRequestApplicantQueryFailed 获取申请人资料失败。
	FriendRequestApplicantQueryFailed int = 50008

	// FriendRequestIDEmpty 好友申请 id 为空。
	FriendRequestIDEmpty int = 50009

	// FriendRequestNotFound 好友申请不存在。
	FriendRequestNotFound int = 50010

	// FriendRequestHandleFailed 处理好友申请失败。
	FriendRequestHandleFailed int = 50011

	// FriendTrainingVisibilityUpdateFailed 更新训练公开状态失败。
	FriendTrainingVisibilityUpdateFailed int = 50012

	// FriendTrainingSnapshotEmpty 训练公开快照为空。
	FriendTrainingSnapshotEmpty int = 50013

	// FriendTrainingSnapshotUploadFailed 上传训练公开快照失败。
	FriendTrainingSnapshotUploadFailed int = 50014

	// FriendEntryStatusQueryFailed 获取好友入口状态失败。
	FriendEntryStatusQueryFailed int = 50015

	// FriendUIDEmpty uid 为空。
	FriendUIDEmpty int = 50016

	// FriendNotFound 好友不存在。
	FriendNotFound int = 50017

	// FriendRemarkUpdateFailed 更新好友备注失败。
	FriendRemarkUpdateFailed int = 50018

	// FriendProfileUpdateFailed 更新朋友资料失败。
	FriendProfileUpdateFailed int = 50019

	// BodyPhotoRecordEmpty 照片索引记录为空。
	BodyPhotoRecordEmpty int = 60001

	// BodyPhotoClientRecordIDEmpty 照片索引客户端记录 id 为空。
	BodyPhotoClientRecordIDEmpty int = 60002

	// BodyPhotoAssetIDEmpty 照片资源 id 为空。
	BodyPhotoAssetIDEmpty int = 60003

	// BodyPhotoKindEmpty 照片类型为空。
	BodyPhotoKindEmpty int = 60004

	// BodyPhotoRecordAtEmpty 照片记录时间为空。
	BodyPhotoRecordAtEmpty int = 60005

	// BodyPhotoSaveFailed 保存照片索引失败。
	BodyPhotoSaveFailed int = 60006

	// BodyPhotoDeleteKeyEmpty 删除照片索引缺少 id 或客户端记录 id。
	BodyPhotoDeleteKeyEmpty int = 60007

	// BodyPhotoNotFound 照片索引不存在。
	BodyPhotoNotFound int = 60008

	// BodyPhotoDeleteFailed 删除照片索引失败。
	BodyPhotoDeleteFailed int = 60009

	// BodyPhotoDailyLimitExceeded 当天照片数量超过上限。
	BodyPhotoDailyLimitExceeded int = 60010

	// BodyPhotoDailyCreateLimitExceeded 当天照片新增数量超过上限。
	BodyPhotoDailyCreateLimitExceeded int = 60011

	// VIPStatusQueryFailed 查询 VIP 权益状态失败。
	VIPStatusQueryFailed int = 70001

	// VIPTransactionJWSMissing Apple 交易签名数据为空。
	VIPTransactionJWSMissing int = 70002

	// VIPTransactionVerifyConfigInvalid Apple 交易验签配置无效。
	VIPTransactionVerifyConfigInvalid int = 70003

	// VIPTransactionVerifyFailed Apple 交易验签失败。
	VIPTransactionVerifyFailed int = 70004

	// VIPProductUnsupported VIP 商品 ID 未配置或不支持。
	VIPProductUnsupported int = 70005

	// VIPEntitlementSaveFailed 保存 VIP 权益失败。
	VIPEntitlementSaveFailed int = 70006

	// VIPPurchaseOrderCreateFailed 创建 VIP 支付预订单失败。
	VIPPurchaseOrderCreateFailed int = 70007

	// VIPPurchaseOrderMissing VIP 支付预订单不存在。
	VIPPurchaseOrderMissing int = 70008

	// VIPPurchaseOrderExpired VIP 支付预订单已过期。
	VIPPurchaseOrderExpired int = 70009

	// VIPPurchaseOrderProductMismatch VIP 支付预订单与 Apple 交易商品不一致。
	VIPPurchaseOrderProductMismatch int = 70010

	// VIPPurchaseOrderRequired VIP 支付确认缺少预订单。
	VIPPurchaseOrderRequired int = 70011

	// VIPAppleTransactionAlreadyBound Apple 交易已绑定其他账号。
	VIPAppleTransactionAlreadyBound int = 70012

	// VIPPurchaseOrderTransactionMismatch VIP 支付预订单与 Apple 交易不一致。
	VIPPurchaseOrderTransactionMismatch int = 70013
)
