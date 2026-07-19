package gateway

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	mysqlmodel "spider-server/mysql/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminGrantVIPRequest struct {
	Account         string `json:"account"`
	Lifetime        bool   `json:"lifetime"`
	DurationDays    int64  `json:"duration_days"`
	DurationMinutes int64  `json:"duration_minutes"`
	ExpiresAt       int64  `json:"expires_at"`
	Operator        string `json:"operator"`
	Reason          string `json:"reason"`
}

type adminRevokeVIPRequest struct {
	Account  string `json:"account"`
	Operator string `json:"operator"`
	Reason   string `json:"reason"`
}

type adminResolveClientSyncFailureRequest struct {
	Operator string `json:"operator"`
	Note     string `json:"note"`
}

type adminAppUpdateRequest struct {
	LatestVersion          string `json:"latest_version"`
	MinSupportedVersion    string `json:"min_supported_version"`
	ForceUpdateEnabled     bool   `json:"force_update_enabled"`
	UpdateAvailableEnabled bool   `json:"update_available_enabled"`
	AppStoreURL            string `json:"app_store_url"`
	MessageZhHans          string `json:"message_zh_hans"`
	MessageZhHant          string `json:"message_zh_hant"`
	MessageEn              string `json:"message_en"`
	MessageJa              string `json:"message_ja"`
	MessageKo              string `json:"message_ko"`
}

type adminVIPStatus struct {
	IsVIP     bool       `json:"is_vip"`
	Kind      string     `json:"kind"`
	ExpiresAt *time.Time `json:"expires_at"`
	ProductID string     `json:"product_id"`
	Source    string     `json:"source"`
}

func (s *GatewayServer) registerAdminConsoleRoutes(router *gin.Engine) {
	group := router.Group("/admin-console")
	group.Use(s.adminAuth.middleware())
	group.GET("/health", s.adminHealthHandler)
	group.GET("/overview", s.adminOverviewHandler)
	group.GET("/users/:identifier", s.adminUserHandler)
	group.POST("/vip/grant", s.adminGrantVIPHandler)
	group.POST("/vip/revoke", s.adminRevokeVIPHandler)
	group.GET("/app-update", s.adminGetAppUpdateHandler)
	group.PUT("/app-update", s.adminUpdateAppUpdateHandler)
	group.GET("/payments", s.adminPaymentsHandler)
	group.GET("/paywall-sessions", s.adminPaywallSessionsHandler)
	group.GET("/refunds", s.adminRefundsHandler)
	group.GET("/daily-active", s.adminDailyActiveHandler)
	group.GET("/registrations", s.adminRegistrationsHandler)
	group.GET("/feedback", s.adminFeedbackHandler)
	group.GET("/client-sync-failures", s.adminClientSyncFailuresHandler)
	group.POST("/client-sync-failures/:id/resolve", s.adminResolveClientSyncFailureHandler)
	group.GET("/onboarding-profiles", s.adminOnboardingProfilesHandler)
	group.GET("/friend-profiles", s.adminFriendProfilesHandler)
	group.GET("/shared-content-scores", s.adminSharedContentScoresHandler)
	group.GET("/feature-adoption", s.adminFeatureAdoptionHandler)
	group.GET("/plan-data-users", s.adminPlanDataUsersHandler)
	group.GET("/plan-data-users/:uid", s.adminPlanDataDetailHandler)
	group.GET("/workout-data-users", s.adminWorkoutDataUsersHandler)
	group.GET("/workout-data-users/:uid/sessions", s.adminWorkoutDataSessionsHandler)
}

func (s *GatewayServer) adminHealthHandler(c *gin.Context) {
	adminOK(c, gin.H{"server_time": time.Now()})
}

func (s *GatewayServer) adminOverviewHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, true)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	query.PageSize = 1
	_, activeCount, err := mysqlmodel.ListAdminDailyActivities(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询日活失败")
		return
	}
	_, registrationCount, err := mysqlmodel.ListAdminRegistrations(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询注册数据失败")
		return
	}
	_, paymentCount, err := mysqlmodel.ListAdminPayments(query, mysqlmodel.AdminPaymentSourceAll, "")
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询付费数据失败")
		return
	}
	feedbackQuery := query
	feedbackQuery.From = nil
	feedbackQuery.To = nil
	_, feedbackCount, err := mysqlmodel.ListAdminFeedback(feedbackQuery)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询用户反馈失败")
		return
	}
	appUpdate, err := mysqlmodel.GetAppUpdateConfig(mysqlmodel.AppUpdatePlatformIOS)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询版本更新配置失败")
		return
	}
	adminOK(c, gin.H{
		"daily_active":   activeCount,
		"registrations":  registrationCount,
		"payments":       paymentCount,
		"feedback":       feedbackCount,
		"latest_version": appUpdate.LatestVersion,
	})
}

func (s *GatewayServer) adminUserHandler(c *gin.Context) {
	detail, err := mysqlmodel.GetAdminUserDetail(c.Param("identifier"), time.Now())
	if err != nil {
		if errors.Is(err, mysqlmodel.ErrAdminVIPAccountNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			adminError(c, http.StatusNotFound, "没有找到该用户")
			return
		}
		adminError(c, http.StatusInternalServerError, "查询用户失败")
		return
	}
	adminOK(c, gin.H{
		"uid":                     detail.UID,
		"account":                 detail.Account,
		"nickname":                detail.Nickname,
		"apple_email":             detail.AppleEmail,
		"last_app_enter_at":       detail.LastAppEnterAt,
		"last_system_language":    detail.LastSystemLanguage,
		"last_app_version":        detail.LastAppVersion,
		"register_device_model":   detail.RegisterDeviceModel,
		"register_device_label":   detail.RegisterDeviceLabel,
		"register_ios_version":    detail.RegisterIOSVersion,
		"last_login_device_model": detail.LastLoginDeviceModel,
		"last_login_device_label": detail.LastLoginDeviceLabel,
		"last_login_ios_version":  detail.LastLoginIOSVersion,
		"last_login_at":           detail.LastLoginAt,
		"created_at":              detail.CreatedAt,
		"updated_at":              detail.UpdatedAt,
		"vip":                     toAdminVIPStatus(detail.VIP),
	})
}

func (s *GatewayServer) adminGrantVIPHandler(c *gin.Context) {
	var req adminGrantVIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminError(c, http.StatusBadRequest, "开通参数无效")
		return
	}
	now := time.Now()
	expiresAt := req.ExpiresAt
	if !req.Lifetime && expiresAt <= 0 && req.DurationMinutes > 0 {
		expiresAt = now.Add(time.Duration(req.DurationMinutes) * time.Minute).Unix()
	}
	user, status, err := mysqlmodel.GrantAdminVIPByAccount(
		req.Account,
		req.Lifetime,
		req.DurationDays,
		expiresAt,
		req.Operator,
		req.Reason,
		now,
	)
	if err != nil {
		switch {
		case errors.Is(err, mysqlmodel.ErrAdminVIPAccountNotFound):
			adminError(c, http.StatusNotFound, "没有找到该用户")
		case errors.Is(err, mysqlmodel.ErrAdminVIPDurationInvalid):
			adminError(c, http.StatusBadRequest, "VIP 时长无效")
		default:
			adminError(c, http.StatusInternalServerError, "开通 VIP 失败")
		}
		return
	}
	adminOK(c, gin.H{"uid": user.ID, "account": user.Account, "vip": toAdminVIPStatus(status)})
}

func (s *GatewayServer) adminRevokeVIPHandler(c *gin.Context) {
	var req adminRevokeVIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminError(c, http.StatusBadRequest, "撤销参数无效")
		return
	}
	user, status, err := mysqlmodel.RevokeAdminVIPByAccount(req.Account, req.Operator, req.Reason, time.Now())
	if err != nil {
		if errors.Is(err, mysqlmodel.ErrAdminVIPAccountNotFound) {
			adminError(c, http.StatusNotFound, "没有找到该用户")
			return
		}
		adminError(c, http.StatusInternalServerError, "撤销后台 VIP 失败")
		return
	}
	adminOK(c, gin.H{"uid": user.ID, "account": user.Account, "vip": toAdminVIPStatus(status)})
}

func (s *GatewayServer) adminGetAppUpdateHandler(c *gin.Context) {
	record, err := mysqlmodel.GetAppUpdateConfig(mysqlmodel.AppUpdatePlatformIOS)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询 App 更新配置失败")
		return
	}
	adminOK(c, adminAppUpdateData(record))
}

func (s *GatewayServer) adminUpdateAppUpdateHandler(c *gin.Context) {
	var req adminAppUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminError(c, http.StatusBadRequest, "App 更新配置无效")
		return
	}
	record, err := mysqlmodel.UpsertAppUpdateConfig(mysqlmodel.AppUpdateConfigInput{
		Platform:               mysqlmodel.AppUpdatePlatformIOS,
		LatestVersion:          req.LatestVersion,
		MinSupportedVersion:    req.MinSupportedVersion,
		ForceUpdateEnabled:     req.ForceUpdateEnabled,
		UpdateAvailableEnabled: req.UpdateAvailableEnabled,
		AppStoreURL:            req.AppStoreURL,
		MessageZhHans:          req.MessageZhHans,
		MessageZhHant:          req.MessageZhHant,
		MessageEn:              req.MessageEn,
		MessageJa:              req.MessageJa,
		MessageKo:              req.MessageKo,
	})
	if err != nil {
		adminError(c, http.StatusInternalServerError, "保存 App 更新配置失败")
		return
	}
	adminOK(c, adminAppUpdateData(record))
}

func (s *GatewayServer) adminPaymentsHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminPayments(query, c.Query("source"), c.Query("entry_point"))
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询付费记录失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminPaywallSessionsHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminPaywallSessions(query, c.Query("status"), c.Query("entry_point"))
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询付费墙会话失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminRefundsHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminRefunds(query, c.Query("status"), c.Query("source"))
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询退款记录失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminDailyActiveHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, true)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminDailyActivities(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询日活用户失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminRegistrationsHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, true)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminRegistrations(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询注册用户失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminFeedbackHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminFeedback(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询用户反馈失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminClientSyncFailuresHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminClientSyncFailures(query, c.Query("status"))
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询丢弃任务失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminResolveClientSyncFailureHandler(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		adminError(c, http.StatusBadRequest, "任务 ID 无效")
		return
	}
	var req adminResolveClientSyncFailureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		adminError(c, http.StatusBadRequest, "处理参数无效")
		return
	}
	resolvedAt := time.Now()
	err = mysqlmodel.ResolveAdminClientSyncFailure(id, req.Operator, req.Note, resolvedAt)
	if err != nil {
		switch {
		case errors.Is(err, mysqlmodel.ErrAdminSyncFailureOperatorEmpty):
			adminError(c, http.StatusBadRequest, "处理人不能为空")
		case errors.Is(err, gorm.ErrRecordNotFound):
			adminError(c, http.StatusNotFound, "没有找到该丢弃任务")
		default:
			adminError(c, http.StatusInternalServerError, "标记丢弃任务失败")
		}
		return
	}
	adminOK(c, gin.H{
		"id":          id,
		"status":      mysqlmodel.AdminClientSyncFailureStatusResolved,
		"resolved_at": resolvedAt,
		"resolved_by": strings.TrimSpace(req.Operator),
	})
}

func (s *GatewayServer) adminOnboardingProfilesHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminOnboardingProfiles(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询 Onboard 信息失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminFriendProfilesHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminFriendProfiles(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询好友资料失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminSharedContentScoresHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	var kind int32
	switch strings.ToLower(strings.TrimSpace(c.Query("kind"))) {
	case "", "plan":
		kind = mysqlmodel.FriendSharedContentKindPlan
	case "training":
		kind = mysqlmodel.FriendSharedContentKindTraining
	default:
		adminError(c, http.StatusBadRequest, "积分类型无效")
		return
	}
	items, total, err := mysqlmodel.ListAdminSharedContentScores(query, kind)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询分享积分失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminFeatureAdoptionHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, true)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminDailyFeatureAdoption(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询功能新增数据失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminPlanDataUsersHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminPlanDataUsers(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询计划用户失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminPlanDataDetailHandler(c *gin.Context) {
	uid, ok := adminUIDFromParam(c)
	if !ok {
		return
	}
	folders, err := mysqlmodel.GetAdminPlanDataDetail(uid)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询计划详情失败")
		return
	}
	adminOK(c, gin.H{"uid": uid, "folders": folders})
}

func (s *GatewayServer) adminWorkoutDataUsersHandler(c *gin.Context) {
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminWorkoutDataUsers(query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询训练用户失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func (s *GatewayServer) adminWorkoutDataSessionsHandler(c *gin.Context) {
	uid, ok := adminUIDFromParam(c)
	if !ok {
		return
	}
	query, err := adminPageQueryFromContext(c, false)
	if err != nil {
		adminError(c, http.StatusBadRequest, err.Error())
		return
	}
	items, total, err := mysqlmodel.ListAdminWorkoutSessionDetails(uid, query)
	if err != nil {
		adminError(c, http.StatusInternalServerError, "查询训练详情失败")
		return
	}
	adminPageOK(c, items, total, query)
}

func adminUIDFromParam(c *gin.Context) (uint64, bool) {
	uid, err := strconv.ParseUint(c.Param("uid"), 10, 64)
	if err != nil || uid == 0 {
		adminError(c, http.StatusBadRequest, "用户 ID 无效")
		return 0, false
	}
	return uid, true
}

func adminPageQueryFromContext(c *gin.Context, defaultToday bool) (mysqlmodel.AdminPageQuery, error) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	fromText := strings.TrimSpace(c.Query("from"))
	toText := strings.TrimSpace(c.Query("to"))
	if defaultToday && fromText == "" && toText == "" {
		fromText = time.Now().In(time.Local).Format("2006-01-02")
		toText = fromText
	}
	from, err := parseAdminDate(fromText, false)
	if err != nil {
		return mysqlmodel.AdminPageQuery{}, err
	}
	to, err := parseAdminDate(toText, true)
	if err != nil {
		return mysqlmodel.AdminPageQuery{}, err
	}
	if err := mysqlmodel.ValidateAdminDateRange(from, to); err != nil {
		return mysqlmodel.AdminPageQuery{}, err
	}
	return mysqlmodel.AdminPageQuery{
		Search:   c.Query("search"),
		From:     from,
		To:       to,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func parseAdminDate(value string, endExclusive bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, errors.New("日期格式必须是 YYYY-MM-DD")
	}
	if endExclusive {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return &parsed, nil
}

func toAdminVIPStatus(status mysqlmodel.CurrentVIPStatus) adminVIPStatus {
	return adminVIPStatus{
		IsVIP:     status.IsVIP,
		Kind:      status.Kind,
		ExpiresAt: status.ExpiresAt,
		ProductID: status.ProductID,
		Source:    status.Source,
	}
}

func adminAppUpdateData(record *mysqlmodel.AppUpdateConfig) gin.H {
	if record == nil {
		return gin.H{}
	}
	return gin.H{
		"platform":                 record.Platform,
		"latest_version":           record.LatestVersion,
		"min_supported_version":    record.MinSupportedVersion,
		"force_update_enabled":     record.ForceUpdateEnabled,
		"update_available_enabled": record.UpdateAvailableEnabled,
		"app_store_url":            record.AppStoreURL,
		"message_zh_hans":          record.MessageZhHans,
		"message_zh_hant":          record.MessageZhHant,
		"message_en":               record.MessageEn,
		"message_ja":               record.MessageJa,
		"message_ko":               record.MessageKo,
		"updated_at":               record.UpdatedAt,
	}
}

func adminOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": data})
}

func adminPageOK(c *gin.Context, items any, total int64, query mysqlmodel.AdminPageQuery) {
	adminOK(c, gin.H{"items": items, "total": total, "page": query.Page, "page_size": query.PageSize})
}

func adminError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": status, "message": message})
}
