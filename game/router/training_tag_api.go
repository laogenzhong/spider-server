package router

import (
	"context"
	"errors"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"gorm.io/gorm"
)

// TrainingTagApi 实现训练标签相关 gRPC 接口。
//
// 设计原则：
// 1. 服务端 MySQL 是训练标签和训练绑定关系的主数据源。
// 2. uid 从登录 token/session 对应的 ctx 中获取，客户端不需要主动传 uid。
// 3. 标签库和训练绑定分开管理。
// 4. 保存某次训练标签时采用覆盖式保存：先删除旧绑定，再写入新绑定。
type TrainingTagApi struct {
	pb.UnimplementedTrainingTagServiceServer
}

// NewTrainingTagApi 创建训练标签服务实例。
func NewTrainingTagApi() *TrainingTagApi {
	return &TrainingTagApi{}
}

// CreateTrainingTag 新建用户自定义训练标签。
func (a *TrainingTagApi) CreateTrainingTag(ctx context.Context, req *pb.CreateTrainingTagRequest) (*pb.CreateTrainingTagResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetName() == "" {
		return session.Error(ctx, gamecode.TrainingTagNameEmpty, &pb.CreateTrainingTagResponse{})
	}

	tag, err := mysqlmodel.CreateTrainingTag(uid, req.GetName(), req.GetSortOrder())
	if err != nil {
		return session.Error(ctx, gamecode.TrainingTagCreateFailed, &pb.CreateTrainingTagResponse{})
	}

	return &pb.CreateTrainingTagResponse{
		Tag: convertTrainingTag(tag),
	}, nil
}

// UpdateTrainingTag 修改训练标签。
func (a *TrainingTagApi) UpdateTrainingTag(ctx context.Context, req *pb.UpdateTrainingTagRequest) (*pb.UpdateTrainingTagResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 {
		return session.Error(ctx, gamecode.TrainingTagIDEmpty, &pb.UpdateTrainingTagResponse{})
	}
	if req.GetName() == "" {
		return session.Error(ctx, gamecode.TrainingTagNameEmpty, &pb.UpdateTrainingTagResponse{})
	}

	tag, err := mysqlmodel.UpdateTrainingTag(
		uid,
		req.GetId(),
		req.GetName(),
		req.GetSortOrder(),
		req.GetEnabled(),
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.TrainingTagNotFound, &pb.UpdateTrainingTagResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.TrainingTagUpdateFailed, &pb.UpdateTrainingTagResponse{})
	}

	return &pb.UpdateTrainingTagResponse{
		Tag: convertTrainingTag(tag),
	}, nil
}

// DeleteTrainingTag 删除训练标签。
func (a *TrainingTagApi) DeleteTrainingTag(ctx context.Context, req *pb.DeleteTrainingTagRequest) (*pb.DeleteTrainingTagResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 {
		return session.Error(ctx, gamecode.TrainingTagIDEmpty, &pb.DeleteTrainingTagResponse{})
	}

	if err := mysqlmodel.DeleteTrainingTag(uid, req.GetId()); err != nil {
		return session.Error(ctx, gamecode.TrainingTagDeleteFailed, &pb.DeleteTrainingTagResponse{})
	}

	return &pb.DeleteTrainingTagResponse{Success: true}, nil
}

// ListTrainingTags 获取当前用户可用标签列表。
func (a *TrainingTagApi) ListTrainingTags(ctx context.Context, req *pb.ListTrainingTagsRequest) (*pb.ListTrainingTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	tags, err := mysqlmodel.ListTrainingTags(uid, req.GetOnlyEnabled())
	if err != nil {
		return session.Error(ctx, gamecode.TrainingTagListFailed, &pb.ListTrainingTagsResponse{})
	}

	respTags := make([]*pb.TrainingTag, 0, len(tags))
	for _, tag := range tags {
		respTags = append(respTags, convertTrainingTag(tag))
	}

	return &pb.ListTrainingTagsResponse{
		Tags: respTags,
	}, nil
}

// ReorderTrainingTags 批量调整训练标签排序。
func (a *TrainingTagApi) ReorderTrainingTags(ctx context.Context, req *pb.ReorderTrainingTagsRequest) (*pb.ReorderTrainingTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	items := make([]mysqlmodel.TrainingTagSortItem, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		if item.GetId() == 0 {
			continue
		}

		items = append(items, mysqlmodel.TrainingTagSortItem{
			ID:        item.GetId(),
			SortOrder: item.GetSortOrder(),
		})
	}

	tags, err := mysqlmodel.ReorderTrainingTags(uid, items)
	if err != nil {
		return session.Error(ctx, gamecode.TrainingTagReorderFailed, &pb.ReorderTrainingTagsResponse{})
	}

	respTags := make([]*pb.TrainingTag, 0, len(tags))
	for _, tag := range tags {
		respTags = append(respTags, convertTrainingTag(tag))
	}

	return &pb.ReorderTrainingTagsResponse{
		Tags: respTags,
	}, nil
}

// SaveWorkoutTags 保存某次训练绑定的完整标签列表。
//
// 服务端采用覆盖式保存：
// 1. 删除当前 workout 的旧标签绑定。
// 2. 根据 tag_ids 写入新的绑定关系。
// 3. 返回最新绑定列表。
func (a *TrainingTagApi) SaveWorkoutTags(ctx context.Context, req *pb.SaveWorkoutTagsRequest) (*pb.SaveWorkoutTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetWorkoutUuid() == "" && (req.GetWorkoutStartAt() == 0 || req.GetWorkoutEndAt() == 0) {
		return session.Error(ctx, gamecode.WorkoutTagsTargetEmpty, &pb.SaveWorkoutTagsResponse{})
	}

	bindings, err := mysqlmodel.SaveWorkoutTags(
		uid,
		req.GetWorkoutUuid(),
		req.GetWorkoutStartAt(),
		req.GetWorkoutEndAt(),
		req.GetWorkoutType(),
		req.GetTagIds(),
	)
	if err != nil {
		return session.Error(ctx, gamecode.WorkoutTagsSaveFailed, &pb.SaveWorkoutTagsResponse{})
	}

	respBindings := make([]*pb.WorkoutTagBinding, 0, len(bindings))
	for _, binding := range bindings {
		respBindings = append(respBindings, convertWorkoutTagBinding(binding))
	}

	return &pb.SaveWorkoutTagsResponse{
		Bindings: respBindings,
	}, nil
}

// GetWorkoutTags 获取某次训练绑定的标签列表。
func (a *TrainingTagApi) GetWorkoutTags(ctx context.Context, req *pb.GetWorkoutTagsRequest) (*pb.GetWorkoutTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetWorkoutUuid() == "" && (req.GetWorkoutStartAt() == 0 || req.GetWorkoutEndAt() == 0) {
		return session.Error(ctx, gamecode.WorkoutTagsTargetEmpty, &pb.GetWorkoutTagsResponse{})
	}

	bindings, err := mysqlmodel.GetWorkoutTags(
		uid,
		req.GetWorkoutUuid(),
		req.GetWorkoutStartAt(),
		req.GetWorkoutEndAt(),
	)
	if err != nil {
		return session.Error(ctx, gamecode.WorkoutTagsQueryFailed, &pb.GetWorkoutTagsResponse{})
	}

	respBindings := make([]*pb.WorkoutTagBinding, 0, len(bindings))
	for _, binding := range bindings {
		respBindings = append(respBindings, convertWorkoutTagBinding(binding))
	}

	return &pb.GetWorkoutTagsResponse{
		Bindings: respBindings,
	}, nil
}

// DeleteWorkoutTags 删除某次训练绑定的全部标签。
func (a *TrainingTagApi) DeleteWorkoutTags(ctx context.Context, req *pb.DeleteWorkoutTagsRequest) (*pb.DeleteWorkoutTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetWorkoutUuid() == "" && (req.GetWorkoutStartAt() == 0 || req.GetWorkoutEndAt() == 0) {
		return session.Error(ctx, gamecode.WorkoutTagsTargetEmpty, &pb.DeleteWorkoutTagsResponse{})
	}

	if err := mysqlmodel.DeleteWorkoutTags(
		uid,
		req.GetWorkoutUuid(),
		req.GetWorkoutStartAt(),
		req.GetWorkoutEndAt(),
	); err != nil {
		return session.Error(ctx, gamecode.WorkoutTagsDeleteFailed, &pb.DeleteWorkoutTagsResponse{})
	}

	return &pb.DeleteWorkoutTagsResponse{Success: true}, nil
}

// ListDailyWorkoutTags 获取某一天内所有训练及其标签。
func (a *TrainingTagApi) ListDailyWorkoutTags(ctx context.Context, req *pb.ListDailyWorkoutTagsRequest) (*pb.ListDailyWorkoutTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetRecordDate() == "" {
		return session.Error(ctx, gamecode.WorkoutTagsRecordDateEmpty, &pb.ListDailyWorkoutTagsResponse{})
	}

	workouts, err := mysqlmodel.ListDailyWorkoutTags(uid, req.GetRecordDate())
	if err != nil {
		return session.Error(ctx, gamecode.WorkoutTagsDailyQueryFailed, &pb.ListDailyWorkoutTagsResponse{})
	}

	respWorkouts := make([]*pb.DailyWorkoutTags, 0, len(workouts))
	for _, workout := range workouts {
		respWorkouts = append(respWorkouts, convertDailyWorkoutTags(workout))
	}

	return &pb.ListDailyWorkoutTagsResponse{
		Workouts: respWorkouts,
	}, nil
}

// ListRangeWorkoutTags 获取一段日期内按天聚合的训练标签摘要。
func (a *TrainingTagApi) ListRangeWorkoutTags(ctx context.Context, req *pb.ListRangeWorkoutTagsRequest) (*pb.ListRangeWorkoutTagsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetStartDate() == "" || req.GetEndDate() == "" {
		return session.Error(ctx, gamecode.WorkoutTagsDateRangeEmpty, &pb.ListRangeWorkoutTagsResponse{})
	}

	days, err := mysqlmodel.ListRangeWorkoutTags(uid, req.GetStartDate(), req.GetEndDate())
	if err != nil {
		return session.Error(ctx, gamecode.WorkoutTagsRangeQueryFailed, &pb.ListRangeWorkoutTagsResponse{})
	}

	respDays := make([]*pb.DailyTrainingTagSummary, 0, len(days))
	for _, day := range days {
		respDays = append(respDays, convertDailyTrainingTagSummary(day))
	}

	return &pb.ListRangeWorkoutTagsResponse{
		Days: respDays,
	}, nil
}

// convertTrainingTag 将 MySQL 训练标签模型转换为 pb 模型。
func convertTrainingTag(tag *mysqlmodel.TrainingTag) *pb.TrainingTag {
	if tag == nil {
		return nil
	}

	return &pb.TrainingTag{
		Id:        tag.ID,
		Uid:       tag.UID,
		Name:      tag.Name,
		Type:      pb.TrainingTagType(tag.Type),
		SortOrder: tag.SortOrder,
		Enabled:   tag.Enabled,
		CreatedAt: millis(tag.CreatedAt),
		UpdatedAt: millis(tag.UpdatedAt),
	}
}

// convertWorkoutTagBinding 将 MySQL 训练标签绑定模型转换为 pb 模型。
func convertWorkoutTagBinding(binding *mysqlmodel.WorkoutTagBinding) *pb.WorkoutTagBinding {
	if binding == nil {
		return nil
	}

	return &pb.WorkoutTagBinding{
		Id:             binding.ID,
		Uid:            binding.UID,
		WorkoutUuid:    binding.WorkoutUUID,
		WorkoutStartAt: binding.WorkoutStartAt,
		WorkoutEndAt:   binding.WorkoutEndAt,
		WorkoutType:    binding.WorkoutType,
		TagId:          binding.TagID,
		TagName:        binding.TagName,
		CreatedAt:      millis(binding.CreatedAt),
		UpdatedAt:      millis(binding.UpdatedAt),
	}
}

// convertDailyWorkoutTags 将 MySQL 每日训练标签模型转换为 pb 模型。
func convertDailyWorkoutTags(workout *mysqlmodel.DailyWorkoutTags) *pb.DailyWorkoutTags {
	if workout == nil {
		return nil
	}

	bindings := make([]*pb.WorkoutTagBinding, 0, len(workout.Bindings))
	for _, binding := range workout.Bindings {
		bindings = append(bindings, convertWorkoutTagBinding(binding))
	}

	return &pb.DailyWorkoutTags{
		WorkoutUuid:    workout.WorkoutUUID,
		WorkoutStartAt: workout.WorkoutStartAt,
		WorkoutEndAt:   workout.WorkoutEndAt,
		WorkoutType:    workout.WorkoutType,
		Bindings:       bindings,
	}
}

// convertDailyTrainingTagSummary 将 MySQL 按天训练标签摘要转换为 pb 模型。
func convertDailyTrainingTagSummary(day *mysqlmodel.DailyTrainingTagSummary) *pb.DailyTrainingTagSummary {
	if day == nil {
		return nil
	}

	return &pb.DailyTrainingTagSummary{
		RecordDate: day.RecordDate,
		TagNames:   day.TagNames,
		TagIds:     day.TagIDs,
	}
}
