package router

import (
	"context"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

const (
	friendTrainingSnapshotMaxBytes               = 256 * 1024
	friendTrainingSnapshotMaxDays                = 30
	friendTrainingSnapshotMaxTagsPerDay          = 20
	friendTrainingSnapshotActionDetailDays       = 7
	friendTrainingSnapshotMaxSessionsPerDay      = 4
	friendTrainingSnapshotMaxExercisesPerSession = 12
	friendTrainingSnapshotMaxSetsPerExercise     = 20
	friendTrainingSnapshotMaxWorkoutTags         = 12
	friendTrainingSnapshotMaxStringRunes         = 128
	friendSharedPlanMaxBytes                     = 64 * 1024
	friendSharedPlanMaxExercises                 = 99
	friendSharedPlanMaxSetsPerExercise           = 99
	friendSharedPlanMaxStringRunes               = 256
	friendSharedPlanMaxNoteRunes                 = 1000
)

// FriendApi 实现好友相关 gRPC 接口。
type FriendApi struct {
	pb.UnimplementedFriendServiceServer
}

// ListFriends 获取好友列表。
func (a *FriendApi) ListFriends(ctx context.Context, req *pb.ListFriendsRequest) (*pb.ListFriendsResponse, error) {
	uid := session.GetUser(ctx).UID()

	myProfile, err := mysqlmodel.EnsureFriendProfile(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendProfileQueryFailed, &pb.ListFriendsResponse{})
	}

	profiles, _, err := mysqlmodel.ListFriendProfiles(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendListQueryFailed, &pb.ListFriendsResponse{})
	}

	remarks, err := mysqlmodel.GetFriendRemarks(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendRemarkQueryFailed, &pb.ListFriendsResponse{})
	}

	friends := make([]*pb.FriendListItem, 0, len(profiles))
	for _, profile := range profiles {
		item := convertFriendListItem(profile)
		if remark, ok := remarks[profile.UID]; ok {
			item.Remark = remark
		}
		friends = append(friends, item)
	}

	return &pb.ListFriendsResponse{
		Friends:               friends,
		MyTrainingDataVisible: myProfile.TrainingDataVisible,
	}, nil
}

// AddFriend 通过好友 ID 发送好友申请。
func (a *FriendApi) AddFriend(ctx context.Context, req *pb.AddFriendRequest) (*pb.AddFriendResponse, error) {
	uid := session.GetUser(ctx).UID()
	if req.GetFriendUserId() == "" {
		return session.Error(ctx, gamecode.FriendUserIDEmpty, &pb.AddFriendResponse{})
	}

	message, err := mysqlmodel.AddFriendRequest(uid, req.GetFriendUserId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.FriendUserNotFound, &pb.AddFriendResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.FriendRequestSendFailed, &pb.AddFriendResponse{})
	}

	return &pb.AddFriendResponse{
		Success: true,
		Message: message,
	}, nil
}

// ListFriendRequests 获取当前用户收到的好友申请。
func (a *FriendApi) ListFriendRequests(ctx context.Context, req *pb.ListFriendRequestsRequest) (*pb.ListFriendRequestsResponse, error) {
	uid := session.GetUser(ctx).UID()

	requests, err := mysqlmodel.ListReceivedFriendRequests(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendRequestListFailed, &pb.ListFriendRequestsResponse{})
	}

	respRequests := make([]*pb.FriendRequest, 0, len(requests))
	for _, request := range requests {
		fromProfile, err := mysqlmodel.EnsureFriendProfile(request.FromUID)
		if err != nil {
			return session.Error(ctx, gamecode.FriendRequestApplicantQueryFailed, &pb.ListFriendRequestsResponse{})
		}
		respRequests = append(respRequests, convertFriendRequest(request, fromProfile))
	}

	shares, err := mysqlmodel.ListReceivedFriendPlanShares(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendPlanShareListFailed, &pb.ListFriendRequestsResponse{})
	}
	respShares := make([]*pb.FriendPlanShareNotification, 0, len(shares))
	for _, share := range shares {
		fromProfile, err := mysqlmodel.EnsureFriendProfile(share.FromUID)
		if err != nil {
			return session.Error(ctx, gamecode.FriendPlanShareListFailed, &pb.ListFriendRequestsResponse{})
		}
		plan, err := mysqlmodel.ParseFriendSharedPlan(share.PlanJSON)
		if err != nil {
			return session.Error(ctx, gamecode.FriendPlanShareListFailed, &pb.ListFriendRequestsResponse{})
		}
		respShares = append(respShares, convertFriendPlanShare(share, fromProfile, plan))
	}

	return &pb.ListFriendRequestsResponse{Requests: respRequests, PlanShares: respShares}, nil
}

// HandleFriendRequest 同意或拒绝好友申请。
func (a *FriendApi) HandleFriendRequest(ctx context.Context, req *pb.HandleFriendRequestRequest) (*pb.HandleFriendRequestResponse, error) {
	uid := session.GetUser(ctx).UID()
	if req.GetRequestId() == "" {
		return session.Error(ctx, gamecode.FriendRequestIDEmpty, &pb.HandleFriendRequestResponse{})
	}

	err := mysqlmodel.HandleFriendRequest(uid, req.GetRequestId(), req.GetAccept())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.FriendRequestNotFound, &pb.HandleFriendRequestResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.FriendRequestHandleFailed, &pb.HandleFriendRequestResponse{})
	}

	return &pb.HandleFriendRequestResponse{Success: true}, nil
}

// SendFriendPlanShare 给一位现有好友发送不可变计划快照。
func (a *FriendApi) SendFriendPlanShare(ctx context.Context, req *pb.SendFriendPlanShareRequest) (*pb.SendFriendPlanShareResponse, error) {
	uid := session.GetUser(ctx).UID()
	if req.GetToUid() == 0 {
		return session.Error(ctx, gamecode.FriendPlanShareRecipientEmpty, &pb.SendFriendPlanShareResponse{})
	}
	if code := validateFriendSharedPlan(req.GetClientShareId(), req.GetPlan()); code != 0 {
		return session.Error(ctx, code, &pb.SendFriendPlanShareResponse{})
	}
	record, err := mysqlmodel.SendFriendPlanShare(uid, req.GetToUid(), req.GetClientShareId(), friendSharedPlanFromPB(req.GetPlan()))
	if errors.Is(err, mysqlmodel.ErrFriendPlanShareNotFriend) {
		return session.Error(ctx, gamecode.FriendPlanShareNotFriend, &pb.SendFriendPlanShareResponse{})
	}
	if errors.Is(err, mysqlmodel.ErrFriendPlanSharePendingLimit) {
		return session.Error(ctx, gamecode.FriendPlanSharePendingLimit, &pb.SendFriendPlanShareResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.FriendPlanShareSendFailed, &pb.SendFriendPlanShareResponse{})
	}
	return &pb.SendFriendPlanShareResponse{Success: true, ShareId: requestIDString(record.ID)}, nil
}

// HandleFriendPlanShare 将通知按“已使用”或“已忽略”记录原因后软删除。
func (a *FriendApi) HandleFriendPlanShare(ctx context.Context, req *pb.HandleFriendPlanShareRequest) (*pb.HandleFriendPlanShareResponse, error) {
	uid := session.GetUser(ctx).UID()
	if req.GetShareId() == "" {
		return session.Error(ctx, gamecode.FriendPlanShareIDEmpty, &pb.HandleFriendPlanShareResponse{})
	}
	disposition := int32(req.GetDisposition())
	if disposition != mysqlmodel.FriendPlanShareDispositionUsed && disposition != mysqlmodel.FriendPlanShareDispositionIgnored {
		return session.Error(ctx, gamecode.FriendPlanShareDispositionInvalid, &pb.HandleFriendPlanShareResponse{})
	}
	if err := mysqlmodel.HandleFriendPlanShare(uid, req.GetShareId(), disposition); errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.FriendPlanShareNotFound, &pb.HandleFriendPlanShareResponse{})
	} else if err != nil {
		return session.Error(ctx, gamecode.FriendPlanShareHandleFailed, &pb.HandleFriendPlanShareResponse{})
	}
	return &pb.HandleFriendPlanShareResponse{Success: true}, nil
}

// RecordFriendTrainingUse records one confirmed copy of a friend's training.
func (a *FriendApi) RecordFriendTrainingUse(ctx context.Context, req *pb.RecordFriendTrainingUseRequest) (*pb.RecordFriendTrainingUseResponse, error) {
	uid := session.GetUser(ctx).UID()
	clientEventID := req.GetClientEventId()
	if clientEventID == "" || utf8.RuneCountInString(clientEventID) > 64 {
		return session.Error(ctx, gamecode.FriendTrainingUseEventInvalid, &pb.RecordFriendTrainingUseResponse{})
	}
	if req.GetSourceUid() == 0 {
		return session.Error(ctx, gamecode.FriendTrainingUseSourceEmpty, &pb.RecordFriendTrainingUseResponse{})
	}
	trainingSessionID := req.GetTrainingSessionId()
	if trainingSessionID == "" || utf8.RuneCountInString(trainingSessionID) > friendTrainingSnapshotMaxStringRunes {
		return session.Error(ctx, gamecode.FriendTrainingUseSessionInvalid, &pb.RecordFriendTrainingUseResponse{})
	}
	err := mysqlmodel.RecordFriendTrainingUse(uid, clientEventID, req.GetSourceUid(), trainingSessionID)
	if errors.Is(err, mysqlmodel.ErrFriendTrainingUseNotFriend) {
		return session.Error(ctx, gamecode.FriendTrainingUseNotFriend, &pb.RecordFriendTrainingUseResponse{})
	}
	if errors.Is(err, mysqlmodel.ErrFriendTrainingUseUnavailable) {
		return session.Error(ctx, gamecode.FriendTrainingUseUnavailable, &pb.RecordFriendTrainingUseResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.FriendTrainingUseRecordFailed, &pb.RecordFriendTrainingUseResponse{})
	}
	return &pb.RecordFriendTrainingUseResponse{Success: true}, nil
}

// UpdateTrainingDataVisibility 设置当前用户训练数据公开状态。
func (a *FriendApi) UpdateTrainingDataVisibility(ctx context.Context, req *pb.UpdateTrainingDataVisibilityRequest) (*pb.UpdateTrainingDataVisibilityResponse, error) {
	uid := session.GetUser(ctx).UID()
	snapshot := req.GetSnapshot()

	var sparkDays int32
	var days []mysqlmodel.FriendTrainingDaySummaryRecord
	var updatedAt int64
	if snapshot != nil {
		if code := validateFriendTrainingSnapshot(snapshot); code != 0 {
			return session.Error(ctx, code, &pb.UpdateTrainingDataVisibilityResponse{})
		}
		sparkDays = snapshot.GetSparkDays()
		days = friendTrainingDaysFromPB(snapshot.GetRecentTrainingDays())
		updatedAt = snapshot.GetUpdatedAt()
	}

	_, err := mysqlmodel.UpdateTrainingDataVisibility(uid, req.GetVisible(), sparkDays, days, updatedAt)
	if err != nil {
		return session.Error(ctx, gamecode.FriendTrainingVisibilityUpdateFailed, &pb.UpdateTrainingDataVisibilityResponse{})
	}

	return &pb.UpdateTrainingDataVisibilityResponse{Visible: req.GetVisible()}, nil
}

// UploadMyTrainingPublicSnapshot 上传当前用户最新公开训练快照。
func (a *FriendApi) UploadMyTrainingPublicSnapshot(ctx context.Context, req *pb.UploadMyTrainingPublicSnapshotRequest) (*pb.UploadMyTrainingPublicSnapshotResponse, error) {
	uid := session.GetUser(ctx).UID()
	snapshot := req.GetSnapshot()
	if snapshot == nil {
		return session.Error(ctx, gamecode.FriendTrainingSnapshotEmpty, &pb.UploadMyTrainingPublicSnapshotResponse{})
	}
	if code := validateFriendTrainingSnapshot(snapshot); code != 0 {
		return session.Error(ctx, code, &pb.UploadMyTrainingPublicSnapshotResponse{})
	}

	err := mysqlmodel.UploadTrainingPublicSnapshot(
		uid,
		snapshot.GetSparkDays(),
		friendTrainingDaysFromPB(snapshot.GetRecentTrainingDays()),
		snapshot.GetUpdatedAt(),
	)
	if err != nil {
		return session.Error(ctx, gamecode.FriendTrainingSnapshotUploadFailed, &pb.UploadMyTrainingPublicSnapshotResponse{})
	}

	return &pb.UploadMyTrainingPublicSnapshotResponse{Success: true}, nil
}

// GetFriendEntryStatus 获取好友入口红点/蓝点状态。
func (a *FriendApi) GetFriendEntryStatus(ctx context.Context, req *pb.GetFriendEntryStatusRequest) (*pb.GetFriendEntryStatusResponse, error) {
	uid := session.GetUser(ctx).UID()

	profile, err := mysqlmodel.EnsureFriendProfile(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendProfileQueryFailed, &pb.GetFriendEntryStatusResponse{})
	}
	pendingCount, err := mysqlmodel.CountPendingFriendRequests(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendEntryStatusQueryFailed, &pb.GetFriendEntryStatusResponse{})
	}
	pendingPlanShareCount, err := mysqlmodel.CountPendingFriendPlanShares(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendEntryStatusQueryFailed, &pb.GetFriendEntryStatusResponse{})
	}
	pendingNotificationCount := pendingCount + pendingPlanShareCount

	return &pb.GetFriendEntryStatusResponse{
		HasPendingRequest:        pendingCount > 0,
		PendingRequestCount:      int32(pendingCount),
		MyTrainingDataVisible:    profile.TrainingDataVisible,
		HasPendingNotification:   pendingNotificationCount > 0,
		PendingNotificationCount: int32(pendingNotificationCount),
	}, nil
}

// GetFriendProfile 获取某个好友的完整资料。
func (a *FriendApi) GetFriendProfile(ctx context.Context, req *pb.GetFriendProfileRequest) (*pb.GetFriendProfileResponse, error) {
	uid := session.GetUser(ctx).UID()
	friendUID := req.GetUid()

	if friendUID == 0 {
		return session.Error(ctx, gamecode.FriendUIDEmpty, &pb.GetFriendProfileResponse{})
	}

	profiles, relationCreatedAt, err := mysqlmodel.ListFriendProfiles(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendListQueryFailed, &pb.GetFriendProfileResponse{})
	}

	var target *mysqlmodel.FriendProfileRecord
	var createdAt int64

	for _, profile := range profiles {
		if profile.UID == friendUID {
			target = profile
			createdAt = relationCreatedAt[profile.UID]
			break
		}
	}

	if target == nil {
		return session.Error(ctx, gamecode.FriendNotFound, &pb.GetFriendProfileResponse{})
	}

	return &pb.GetFriendProfileResponse{
		Profile: convertFriendProfile(target, createdAt),
	}, nil
}

// GetMyFriendProfile 获取当前用户自己的朋友资料。
func (a *FriendApi) GetMyFriendProfile(ctx context.Context, req *pb.GetMyFriendProfileRequest) (*pb.GetMyFriendProfileResponse, error) {
	uid := session.GetUser(ctx).UID()

	profile, err := mysqlmodel.EnsureFriendProfile(uid)
	if err != nil {
		return session.Error(ctx, gamecode.FriendProfileQueryFailed, &pb.GetMyFriendProfileResponse{})
	}

	return &pb.GetMyFriendProfileResponse{
		Profile: convertFriendProfile(profile, profile.CreatedAt.UnixMilli()),
	}, nil
}

// UpdateFriendRemark 修改好友备注名。
func (a *FriendApi) UpdateFriendRemark(ctx context.Context, req *pb.UpdateFriendRemarkRequest) (*pb.UpdateFriendRemarkResponse, error) {
	uid := session.GetUser(ctx).UID()
	friendUID := req.GetUid()

	if friendUID == 0 {
		return session.Error(ctx, gamecode.FriendUIDEmpty, &pb.UpdateFriendRemarkResponse{})
	}

	err := mysqlmodel.UpdateFriendRemark(uid, friendUID, req.GetRemark())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.FriendNotFound, &pb.UpdateFriendRemarkResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.FriendRemarkUpdateFailed, &pb.UpdateFriendRemarkResponse{})
	}

	return &pb.UpdateFriendRemarkResponse{Success: true}, nil
}

// UpdateMyFriendProfile 修改当前用户自己的朋友资料。
func (a *FriendApi) UpdateMyFriendProfile(ctx context.Context, req *pb.UpdateMyFriendProfileRequest) (*pb.UpdateMyFriendProfileResponse, error) {
	uid := session.GetUser(ctx).UID()

	profile, err := mysqlmodel.UpdateFriendProfile(
		uid,
		req.GetNickname(),
		req.GetAvatarSymbol(),
		req.GetBio(),
		req.GetPlanTitle(),
		req.GetPlanDescription(),
	)
	if err != nil {
		return session.Error(ctx, gamecode.FriendProfileUpdateFailed, &pb.UpdateMyFriendProfileResponse{})
	}

	return &pb.UpdateMyFriendProfileResponse{
		Profile: convertFriendProfile(profile, profile.CreatedAt.UnixMilli()),
	}, nil
}

func convertFriendListItem(profile *mysqlmodel.FriendProfileRecord) *pb.FriendListItem {
	if profile == nil {
		return nil
	}

	sparkDays := int32(0)
	if profile.TrainingDataVisible {
		sparkDays = profile.SparkDays
	}

	return &pb.FriendListItem{
		Uid:                 profile.UID,
		UserId:              profile.UserID,
		Nickname:            profile.Nickname,
		AvatarSymbol:        profile.AvatarSymbol,
		Bio:                 profile.Bio,
		TrainingDataVisible: profile.TrainingDataVisible,
		SparkDays:           sparkDays,
		SnapshotUpdatedAt:   profile.SnapshotUpdatedAt,
	}
}

func convertFriendProfile(profile *mysqlmodel.FriendProfileRecord, relationCreatedAt int64) *pb.FriendProfile {
	if profile == nil {
		return nil
	}
	var recentTrainingDays []*pb.FriendTrainingDaySummary
	if profile.TrainingDataVisible {
		recentTrainingDays = friendTrainingDaysToPB(mysqlmodel.ParseFriendTrainingDays(profile.RecentTrainingJSON))
	}

	return &pb.FriendProfile{
		Uid:                profile.UID,
		UserId:             profile.UserID,
		Nickname:           profile.Nickname,
		AvatarSymbol:       profile.AvatarSymbol,
		Bio:                profile.Bio,
		PlanTitle:          profile.PlanTitle,
		PlanDescription:    profile.PlanDescription,
		RecentTrainingDays: recentTrainingDays,
		CreatedAt:          relationCreatedAt,
		SnapshotUpdatedAt:  profile.SnapshotUpdatedAt,
	}
}

func convertFriendRequest(request *mysqlmodel.FriendRequestRecord, fromProfile *mysqlmodel.FriendProfileRecord) *pb.FriendRequest {
	if request == nil {
		return nil
	}

	return &pb.FriendRequest{
		Id:           requestIDString(request.ID),
		FromUid:      request.FromUID,
		FromUserId:   fromProfile.UserID,
		Nickname:     fromProfile.Nickname,
		AvatarSymbol: fromProfile.AvatarSymbol,
		Message:      request.Message,
		Status:       pb.FriendRequestStatus(request.Status),
		CreatedAt:    request.CreatedAt.UnixMilli(),
		HandledAt:    request.HandledAt,
	}
}

func convertFriendPlanShare(share *mysqlmodel.FriendPlanShareRecord, fromProfile *mysqlmodel.FriendProfileRecord, plan mysqlmodel.FriendSharedPlanRecord) *pb.FriendPlanShareNotification {
	return &pb.FriendPlanShareNotification{
		Id:           requestIDString(share.ID),
		FromUid:      share.FromUID,
		FromUserId:   fromProfile.UserID,
		Nickname:     fromProfile.Nickname,
		AvatarSymbol: fromProfile.AvatarSymbol,
		Plan:         friendSharedPlanToPB(plan),
		CreatedAt:    share.CreatedAt.UnixMilli(),
	}
}

func friendSharedPlanFromPB(plan *pb.FriendSharedPlan) mysqlmodel.FriendSharedPlanRecord {
	if plan == nil {
		return mysqlmodel.FriendSharedPlanRecord{}
	}
	exercises := make([]mysqlmodel.FriendSharedPlanExerciseRecord, 0, len(plan.GetExercises()))
	for _, exercise := range plan.GetExercises() {
		if exercise == nil {
			continue
		}
		sets := make([]mysqlmodel.FriendSharedPlanSetRecord, 0, len(exercise.GetSets()))
		for _, set := range exercise.GetSets() {
			if set == nil {
				continue
			}
			sets = append(sets, mysqlmodel.FriendSharedPlanSetRecord{
				WeightText: set.GetWeightText(),
				RepsText:   set.GetRepsText(),
			})
		}
		exercises = append(exercises, mysqlmodel.FriendSharedPlanExerciseRecord{
			ExerciseID:           exercise.GetExerciseId(),
			NameKey:              exercise.GetNameKey(),
			NameSnapshot:         exercise.GetNameSnapshot(),
			CategoryKey:          exercise.GetCategoryKey(),
			TypeKey:              exercise.GetTypeKey(),
			DisplayTypeKey:       exercise.GetDisplayTypeKey(),
			CustomName:           exercise.GetCustomName(),
			CustomSubcategoryKey: exercise.GetCustomSubcategoryKey(),
			CustomIntroduction:   exercise.GetCustomIntroduction(),
			Note:                 exercise.GetNote(),
			SetCount:             exercise.GetSetCount(),
			WeightUnit:           exercise.GetWeightUnit(),
			Sets:                 sets,
		})
	}
	return mysqlmodel.FriendSharedPlanRecord{Title: plan.GetTitle(), SourcePlanID: plan.GetSourcePlanId(), Exercises: exercises}
}

func friendSharedPlanToPB(plan mysqlmodel.FriendSharedPlanRecord) *pb.FriendSharedPlan {
	exercises := make([]*pb.FriendSharedPlanExercise, 0, len(plan.Exercises))
	for _, exercise := range plan.Exercises {
		sets := make([]*pb.FriendSharedPlanSet, 0, len(exercise.Sets))
		for _, set := range exercise.Sets {
			sets = append(sets, &pb.FriendSharedPlanSet{WeightText: set.WeightText, RepsText: set.RepsText})
		}
		exercises = append(exercises, &pb.FriendSharedPlanExercise{
			ExerciseId:           exercise.ExerciseID,
			NameKey:              exercise.NameKey,
			NameSnapshot:         exercise.NameSnapshot,
			CategoryKey:          exercise.CategoryKey,
			TypeKey:              exercise.TypeKey,
			DisplayTypeKey:       exercise.DisplayTypeKey,
			CustomName:           exercise.CustomName,
			CustomSubcategoryKey: exercise.CustomSubcategoryKey,
			CustomIntroduction:   exercise.CustomIntroduction,
			Note:                 exercise.Note,
			SetCount:             exercise.SetCount,
			WeightUnit:           exercise.WeightUnit,
			Sets:                 sets,
		})
	}
	return &pb.FriendSharedPlan{Title: plan.Title, SourcePlanId: plan.SourcePlanID, Exercises: exercises}
}

func friendTrainingDaysFromPB(days []*pb.FriendTrainingDaySummary) []mysqlmodel.FriendTrainingDaySummaryRecord {
	result := make([]mysqlmodel.FriendTrainingDaySummaryRecord, 0, len(days))
	for _, day := range days {
		if day == nil {
			continue
		}
		tags := make([]mysqlmodel.FriendTrainingTagStatRecord, 0, len(day.GetTags()))
		for _, tag := range day.GetTags() {
			if tag == nil {
				continue
			}
			tags = append(tags, mysqlmodel.FriendTrainingTagStatRecord{
				Name:     tag.GetName(),
				Calories: tag.GetCalories(),
			})
		}
		result = append(result, mysqlmodel.FriendTrainingDaySummaryRecord{
			RecordDate:             day.GetRecordDate(),
			Tags:                   tags,
			Calories:               day.GetCalories(),
			ActionTrainingSessions: friendActionTrainingSessionsFromPB(day.GetActionTrainingSessions()),
		})
	}
	return result
}

func friendTrainingDaysToPB(days []mysqlmodel.FriendTrainingDaySummaryRecord) []*pb.FriendTrainingDaySummary {
	result := make([]*pb.FriendTrainingDaySummary, 0, len(days))
	for _, day := range days {
		tags := make([]*pb.FriendTrainingTagStat, 0, len(day.Tags))
		for _, tag := range day.Tags {
			tags = append(tags, &pb.FriendTrainingTagStat{
				Name:     tag.Name,
				Calories: tag.Calories,
			})
		}
		result = append(result, &pb.FriendTrainingDaySummary{
			RecordDate:             day.RecordDate,
			Tags:                   tags,
			Calories:               day.Calories,
			ActionTrainingSessions: friendActionTrainingSessionsToPB(day.ActionTrainingSessions),
		})
	}
	return result
}

func friendActionTrainingSessionsFromPB(sessions []*pb.FriendActionTrainingSession) []mysqlmodel.FriendActionTrainingSessionRecord {
	result := make([]mysqlmodel.FriendActionTrainingSessionRecord, 0, len(sessions))
	for _, training := range sessions {
		if training == nil {
			continue
		}
		exercises := make([]mysqlmodel.FriendActionExerciseSummaryRecord, 0, len(training.GetExercises()))
		for _, exercise := range training.GetExercises() {
			if exercise == nil {
				continue
			}
			sets := make([]mysqlmodel.FriendActionSetSummaryRecord, 0, len(exercise.GetSets()))
			for _, set := range exercise.GetSets() {
				if set == nil {
					continue
				}
				sets = append(sets, mysqlmodel.FriendActionSetSummaryRecord{
					WeightX10:  set.GetWeightX10(),
					WeightUnit: int32(set.GetWeightUnit()),
					Reps:       set.GetReps(),
				})
			}
			exercises = append(exercises, mysqlmodel.FriendActionExerciseSummaryRecord{
				ExerciseID:           exercise.GetExerciseId(),
				NameKey:              exercise.GetNameKey(),
				NameSnapshot:         exercise.GetNameSnapshot(),
				CategoryKey:          exercise.GetCategoryKey(),
				TypeKey:              exercise.GetTypeKey(),
				CustomName:           exercise.GetCustomName(),
				CustomSubcategoryKey: exercise.GetCustomSubcategoryKey(),
				CustomIntroduction:   exercise.GetCustomIntroduction(),
				Sets:                 sets,
			})
		}

		var boundWorkout *mysqlmodel.FriendBoundWorkoutSummaryRecord
		if training.BoundWorkout != nil {
			remote := training.GetBoundWorkout()
			boundWorkout = &mysqlmodel.FriendBoundWorkoutSummaryRecord{
				WorkoutType:  remote.GetWorkoutType(),
				StartAt:      remote.GetStartAt(),
				EndAt:        remote.GetEndAt(),
				DurationSecs: remote.GetDurationSeconds(),
				EnergyKcal:   remote.GetEnergyKcal(),
				Tags:         append([]string(nil), remote.GetTags()...),
			}
			if remote.GetHasDistance() {
				distance := remote.GetDistanceMeters()
				boundWorkout.DistanceMeter = &distance
			}
		}

		result = append(result, mysqlmodel.FriendActionTrainingSessionRecord{
			SessionID:    training.GetSessionId(),
			StartAt:      training.GetStartAt(),
			EndAt:        training.GetEndAt(),
			Kind:         int32(training.GetKind()),
			Exercises:    exercises,
			BoundWorkout: boundWorkout,
		})
	}
	return result
}

func friendActionTrainingSessionsToPB(sessions []mysqlmodel.FriendActionTrainingSessionRecord) []*pb.FriendActionTrainingSession {
	result := make([]*pb.FriendActionTrainingSession, 0, len(sessions))
	for _, training := range sessions {
		exercises := make([]*pb.FriendActionExerciseSummary, 0, len(training.Exercises))
		for _, exercise := range training.Exercises {
			sets := make([]*pb.FriendActionSetSummary, 0, len(exercise.Sets))
			for _, set := range exercise.Sets {
				sets = append(sets, &pb.FriendActionSetSummary{
					WeightX10:  set.WeightX10,
					WeightUnit: pb.FriendActionWeightUnit(set.WeightUnit),
					Reps:       set.Reps,
				})
			}
			exercises = append(exercises, &pb.FriendActionExerciseSummary{
				ExerciseId:           exercise.ExerciseID,
				NameKey:              exercise.NameKey,
				NameSnapshot:         exercise.NameSnapshot,
				CategoryKey:          exercise.CategoryKey,
				TypeKey:              exercise.TypeKey,
				CustomName:           exercise.CustomName,
				CustomSubcategoryKey: exercise.CustomSubcategoryKey,
				CustomIntroduction:   exercise.CustomIntroduction,
				Sets:                 sets,
			})
		}

		remote := &pb.FriendActionTrainingSession{
			SessionId: training.SessionID,
			StartAt:   training.StartAt,
			EndAt:     training.EndAt,
			Kind:      pb.FriendActionTrainingKind(training.Kind),
			Exercises: exercises,
		}
		if training.BoundWorkout != nil {
			bound := training.BoundWorkout
			remote.BoundWorkout = &pb.FriendBoundWorkoutSummary{
				WorkoutType:     bound.WorkoutType,
				StartAt:         bound.StartAt,
				EndAt:           bound.EndAt,
				DurationSeconds: bound.DurationSecs,
				EnergyKcal:      bound.EnergyKcal,
				Tags:            append([]string(nil), bound.Tags...),
			}
			if bound.DistanceMeter != nil {
				remote.BoundWorkout.HasDistance = true
				remote.BoundWorkout.DistanceMeters = *bound.DistanceMeter
			}
		}
		result = append(result, remote)
	}
	return result
}

func validateFriendTrainingSnapshot(snapshot *pb.MyTrainingPublicSnapshot) int {
	if snapshot == nil {
		return gamecode.FriendTrainingSnapshotEmpty
	}
	if proto.Size(snapshot) > friendTrainingSnapshotMaxBytes {
		return gamecode.FriendTrainingSnapshotTooLarge
	}
	if len(snapshot.GetRecentTrainingDays()) > friendTrainingSnapshotMaxDays {
		return gamecode.FriendTrainingSnapshotInvalid
	}
	for dayIndex, day := range snapshot.GetRecentTrainingDays() {
		if day == nil || !validFriendSnapshotString(day.GetRecordDate()) || len(day.GetCalories()) > 32 {
			return gamecode.FriendTrainingSnapshotInvalid
		}
		if _, err := time.Parse("2006-01-02", day.GetRecordDate()); err != nil {
			return gamecode.FriendTrainingSnapshotInvalid
		}
		if len(day.GetTags()) > friendTrainingSnapshotMaxTagsPerDay {
			return gamecode.FriendTrainingSnapshotInvalid
		}
		for _, tag := range day.GetTags() {
			if tag == nil || !validFriendSnapshotString(tag.GetName()) || len(tag.GetCalories()) > 32 {
				return gamecode.FriendTrainingSnapshotInvalid
			}
		}

		sessions := day.GetActionTrainingSessions()
		if dayIndex >= friendTrainingSnapshotActionDetailDays && len(sessions) > 0 {
			return gamecode.FriendTrainingSnapshotInvalid
		}
		if len(sessions) > friendTrainingSnapshotMaxSessionsPerDay {
			return gamecode.FriendTrainingSnapshotInvalid
		}
		for _, training := range sessions {
			if !validFriendActionTrainingSession(training) {
				return gamecode.FriendTrainingSnapshotInvalid
			}
		}
	}
	return 0
}

func validFriendActionTrainingSession(training *pb.FriendActionTrainingSession) bool {
	if training == nil || !validFriendSnapshotString(training.GetSessionId()) || training.GetStartAt() <= 0 || training.GetEndAt() < training.GetStartAt() {
		return false
	}
	if training.GetKind() < pb.FriendActionTrainingKind_FRIEND_ACTION_TRAINING_KIND_STRENGTH || training.GetKind() > pb.FriendActionTrainingKind_FRIEND_ACTION_TRAINING_KIND_MIXED {
		return false
	}
	if len(training.GetExercises()) == 0 || len(training.GetExercises()) > friendTrainingSnapshotMaxExercisesPerSession {
		return false
	}
	for _, exercise := range training.GetExercises() {
		if exercise == nil || !validFriendSnapshotString(exercise.GetExerciseId()) || !validFriendSnapshotString(exercise.GetNameSnapshot()) || !validOptionalFriendSnapshotString(exercise.GetNameKey()) || !validOptionalFriendSnapshotString(exercise.GetCategoryKey()) || !validOptionalFriendSnapshotString(exercise.GetTypeKey()) || !validOptionalFriendSnapshotString(exercise.GetCustomName()) || !validOptionalFriendSnapshotString(exercise.GetCustomSubcategoryKey()) || utf8.RuneCountInString(exercise.GetCustomIntroduction()) > friendSharedPlanMaxNoteRunes {
			return false
		}
		if len(exercise.GetSets()) == 0 || len(exercise.GetSets()) > friendTrainingSnapshotMaxSetsPerExercise {
			return false
		}
		for _, set := range exercise.GetSets() {
			if set == nil || set.GetWeightX10() < 0 || set.GetReps() <= 0 {
				return false
			}
			if set.GetWeightUnit() != pb.FriendActionWeightUnit_FRIEND_ACTION_WEIGHT_UNIT_KG && set.GetWeightUnit() != pb.FriendActionWeightUnit_FRIEND_ACTION_WEIGHT_UNIT_LB {
				return false
			}
		}
	}

	bound := training.GetBoundWorkout()
	if bound == nil {
		return true
	}
	if !validFriendSnapshotString(bound.GetWorkoutType()) || bound.GetStartAt() <= 0 || bound.GetEndAt() < bound.GetStartAt() || bound.GetDurationSeconds() < 0 || bound.GetEnergyKcal() < 0 || bound.GetDistanceMeters() < 0 || len(bound.GetTags()) > friendTrainingSnapshotMaxWorkoutTags {
		return false
	}
	for _, tag := range bound.GetTags() {
		if !validFriendSnapshotString(tag) {
			return false
		}
	}
	return true
}

func validFriendSnapshotString(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= friendTrainingSnapshotMaxStringRunes
}

func validOptionalFriendSnapshotString(value string) bool {
	return utf8.RuneCountInString(value) <= friendTrainingSnapshotMaxStringRunes
}

func validateFriendSharedPlan(clientShareID string, plan *pb.FriendSharedPlan) int {
	if clientShareID == "" || utf8.RuneCountInString(clientShareID) > 64 || plan == nil {
		return gamecode.FriendPlanShareInvalid
	}
	if proto.Size(plan) > friendSharedPlanMaxBytes {
		return gamecode.FriendPlanShareTooLarge
	}
	if !validFriendSharedPlanString(plan.GetTitle()) || len(plan.GetExercises()) == 0 || len(plan.GetExercises()) > friendSharedPlanMaxExercises {
		return gamecode.FriendPlanShareInvalid
	}
	if !validOptionalFriendSharedPlanString(plan.GetSourcePlanId()) {
		return gamecode.FriendPlanShareInvalid
	}
	for _, exercise := range plan.GetExercises() {
		if exercise == nil || !validFriendSharedPlanString(exercise.GetExerciseId()) || !validFriendSharedPlanString(exercise.GetNameSnapshot()) {
			return gamecode.FriendPlanShareInvalid
		}
		if !validOptionalFriendSharedPlanString(exercise.GetNameKey()) || !validOptionalFriendSharedPlanString(exercise.GetCategoryKey()) || !validOptionalFriendSharedPlanString(exercise.GetTypeKey()) || !validOptionalFriendSharedPlanString(exercise.GetDisplayTypeKey()) || !validOptionalFriendSharedPlanString(exercise.GetCustomName()) || !validOptionalFriendSharedPlanString(exercise.GetCustomSubcategoryKey()) || utf8.RuneCountInString(exercise.GetCustomIntroduction()) > friendSharedPlanMaxNoteRunes || utf8.RuneCountInString(exercise.GetNote()) > friendSharedPlanMaxNoteRunes {
			return gamecode.FriendPlanShareInvalid
		}
		if exercise.GetSetCount() <= 0 || exercise.GetSetCount() > friendSharedPlanMaxSetsPerExercise || len(exercise.GetSets()) != int(exercise.GetSetCount()) {
			return gamecode.FriendPlanShareInvalid
		}
		if unit := exercise.GetWeightUnit(); unit != "" && unit != "kg" && unit != "lb" {
			return gamecode.FriendPlanShareInvalid
		}
		for _, set := range exercise.GetSets() {
			if set == nil || !validOptionalFriendSharedPlanString(set.GetWeightText()) || !validOptionalFriendSharedPlanString(set.GetRepsText()) {
				return gamecode.FriendPlanShareInvalid
			}
		}
	}
	return 0
}

func validFriendSharedPlanString(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= friendSharedPlanMaxStringRunes
}

func validOptionalFriendSharedPlanString(value string) bool {
	return utf8.RuneCountInString(value) <= friendSharedPlanMaxStringRunes
}

func requestIDString(id uint64) string {
	return strconv.FormatUint(id, 10)
}
