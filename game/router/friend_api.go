package router

import (
	"context"
	"errors"
	"strconv"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"gorm.io/gorm"
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

	return &pb.ListFriendRequestsResponse{Requests: respRequests}, nil
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

// UpdateTrainingDataVisibility 设置当前用户训练数据公开状态。
func (a *FriendApi) UpdateTrainingDataVisibility(ctx context.Context, req *pb.UpdateTrainingDataVisibilityRequest) (*pb.UpdateTrainingDataVisibilityResponse, error) {
	uid := session.GetUser(ctx).UID()
	snapshot := req.GetSnapshot()

	var sparkDays int32
	var days []mysqlmodel.FriendTrainingDaySummaryRecord
	var updatedAt int64
	if snapshot != nil {
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

	return &pb.GetFriendEntryStatusResponse{
		HasPendingRequest:     pendingCount > 0,
		PendingRequestCount:   int32(pendingCount),
		MyTrainingDataVisible: profile.TrainingDataVisible,
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
			RecordDate: day.GetRecordDate(),
			Tags:       tags,
			Calories:   day.GetCalories(),
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
			RecordDate: day.RecordDate,
			Tags:       tags,
			Calories:   day.Calories,
		})
	}
	return result
}

func requestIDString(id uint64) string {
	return strconv.FormatUint(id, 10)
}
