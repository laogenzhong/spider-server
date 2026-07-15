package router

import (
	"context"
	"errors"
	"strings"
	"time"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"gorm.io/gorm"
)

// ExerciseSetRecordApi 实现动作详情页的重量和次数记录接口。
type ExerciseSetRecordApi struct {
	pb.UnimplementedExerciseSetRecordServiceServer
}

// NewExerciseSetRecordApi 创建动作记录服务实例。
func NewExerciseSetRecordApi() *ExerciseSetRecordApi {
	return &ExerciseSetRecordApi{}
}

// SaveExerciseSetRecord 保存一组动作记录。
func (a *ExerciseSetRecordApi) SaveExerciseSetRecord(ctx context.Context, req *pb.SaveExerciseSetRecordRequest) (*pb.SaveExerciseSetRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if strings.TrimSpace(req.GetExerciseId()) == "" {
		return session.Error(ctx, gamecode.ExerciseRecordExerciseIDEmpty, &pb.SaveExerciseSetRecordResponse{})
	}
	if req.GetWeightX10() < 0 {
		return session.Error(ctx, gamecode.ExerciseRecordWeightInvalid, &pb.SaveExerciseSetRecordResponse{})
	}
	if req.GetReps() <= 0 {
		return session.Error(ctx, gamecode.ExerciseRecordRepsInvalid, &pb.SaveExerciseSetRecordResponse{})
	}
	if !validExerciseWeightUnit(req.GetWeightUnit()) {
		return session.Error(ctx, gamecode.ExerciseRecordWeightUnitInvalid, &pb.SaveExerciseSetRecordResponse{})
	}

	record, err := mysqlmodel.CreateExerciseSetRecord(&mysqlmodel.ExerciseSetRecord{
		UID:                  uid,
		ExerciseID:           strings.TrimSpace(req.GetExerciseId()),
		ExerciseNameKey:      req.GetExerciseNameKey(),
		ExerciseNameSnapshot: req.GetExerciseNameSnapshot(),
		CategoryKey:          req.GetCategoryKey(),
		TypeKey:              req.GetTypeKey(),
		WeightX10:            req.GetWeightX10(),
		WeightUnit:           int32(req.GetWeightUnit()),
		Reps:                 req.GetReps(),
		RecordedAt:           req.GetRecordedAt(),
	})
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseRecordSaveFailed, &pb.SaveExerciseSetRecordResponse{})
	}

	return &pb.SaveExerciseSetRecordResponse{
		Record: mysqlmodel.ExerciseRecordToPB(record),
	}, nil
}

// ListExerciseSetRecords 分页查询某个动作的记录。
func (a *ExerciseSetRecordApi) ListExerciseSetRecords(ctx context.Context, req *pb.ListExerciseSetRecordsRequest) (*pb.ListExerciseSetRecordsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if strings.TrimSpace(req.GetExerciseId()) == "" {
		return session.Error(ctx, gamecode.ExerciseRecordExerciseIDEmpty, &pb.ListExerciseSetRecordsResponse{})
	}

	records, nextCursor, hasMore, err := mysqlmodel.ListExerciseSetRecords(
		uid,
		strings.TrimSpace(req.GetExerciseId()),
		req.GetPageSize(),
		req.GetCursor(),
	)
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseRecordListFailed, &pb.ListExerciseSetRecordsResponse{})
	}

	respRecords := make([]*pb.ExerciseSetRecord, 0, len(records))
	for _, record := range records {
		respRecords = append(respRecords, mysqlmodel.ExerciseRecordToPB(record))
	}

	return &pb.ListExerciseSetRecordsResponse{
		Records:    respRecords,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// ListExerciseSetRecordsByTimeRange 按运动开始和结束时间查询动作记录。
func (a *ExerciseSetRecordApi) ListExerciseSetRecordsByTimeRange(ctx context.Context, req *pb.ListExerciseSetRecordsByTimeRangeRequest) (*pb.ListExerciseSetRecordsByTimeRangeResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetStartAt() <= 0 || req.GetEndAt() <= 0 || req.GetStartAt() > req.GetEndAt() {
		return session.Error(ctx, gamecode.ExerciseRecordTimeRangeInvalid, &pb.ListExerciseSetRecordsByTimeRangeResponse{})
	}

	records, err := mysqlmodel.ListExerciseSetRecordsByTimeRange(uid, req.GetStartAt(), req.GetEndAt())
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseRecordTimeRangeListFailed, &pb.ListExerciseSetRecordsByTimeRangeResponse{})
	}

	respRecords := make([]*pb.ExerciseSetRecord, 0, len(records))
	for _, record := range records {
		respRecords = append(respRecords, mysqlmodel.ExerciseRecordToPB(record))
	}

	return &pb.ListExerciseSetRecordsByTimeRangeResponse{
		Records: respRecords,
	}, nil
}

// UpdateExerciseSetRecord 修改一组动作记录。
func (a *ExerciseSetRecordApi) UpdateExerciseSetRecord(ctx context.Context, req *pb.UpdateExerciseSetRecordRequest) (*pb.UpdateExerciseSetRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 {
		return session.Error(ctx, gamecode.ExerciseRecordIDEmpty, &pb.UpdateExerciseSetRecordResponse{})
	}
	if req.GetWeightX10() < 0 {
		return session.Error(ctx, gamecode.ExerciseRecordWeightInvalid, &pb.UpdateExerciseSetRecordResponse{})
	}
	if req.GetReps() <= 0 {
		return session.Error(ctx, gamecode.ExerciseRecordRepsInvalid, &pb.UpdateExerciseSetRecordResponse{})
	}
	if !validExerciseWeightUnit(req.GetWeightUnit()) {
		return session.Error(ctx, gamecode.ExerciseRecordWeightUnitInvalid, &pb.UpdateExerciseSetRecordResponse{})
	}

	record, err := mysqlmodel.UpdateExerciseSetRecord(&mysqlmodel.ExerciseSetRecord{
		ID:                   req.GetId(),
		UID:                  uid,
		ExerciseID:           strings.TrimSpace(req.GetExerciseId()),
		ExerciseNameKey:      req.GetExerciseNameKey(),
		ExerciseNameSnapshot: req.GetExerciseNameSnapshot(),
		CategoryKey:          req.GetCategoryKey(),
		TypeKey:              req.GetTypeKey(),
		WeightX10:            req.GetWeightX10(),
		WeightUnit:           int32(req.GetWeightUnit()),
		Reps:                 req.GetReps(),
		RecordedAt:           req.GetRecordedAt(),
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.ExerciseRecordNotFound, &pb.UpdateExerciseSetRecordResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseRecordUpdateFailed, &pb.UpdateExerciseSetRecordResponse{})
	}

	return &pb.UpdateExerciseSetRecordResponse{
		Record: mysqlmodel.ExerciseRecordToPB(record),
	}, nil
}

// DeleteExerciseSetRecord 删除一组动作记录。
func (a *ExerciseSetRecordApi) DeleteExerciseSetRecord(ctx context.Context, req *pb.DeleteExerciseSetRecordRequest) (*pb.DeleteExerciseSetRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 {
		return session.Error(ctx, gamecode.ExerciseRecordIDEmpty, &pb.DeleteExerciseSetRecordResponse{})
	}

	err := mysqlmodel.DeleteExerciseSetRecord(uid, req.GetId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.ExerciseRecordNotFound, &pb.DeleteExerciseSetRecordResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseRecordDeleteFailed, &pb.DeleteExerciseSetRecordResponse{})
	}

	return &pb.DeleteExerciseSetRecordResponse{Success: true}, nil
}

// ListTodayExerciseHistory 查询今日动作快捷历史。
func (a *ExerciseSetRecordApi) ListTodayExerciseHistory(ctx context.Context, req *pb.ListTodayExerciseHistoryRequest) (*pb.ListTodayExerciseHistoryResponse, error) {
	uid := session.GetUser(ctx).UID()

	items, err := mysqlmodel.ListTodayExerciseHistory(uid, req.GetRecordDate())
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseRecordTodayListFailed, &pb.ListTodayExerciseHistoryResponse{})
	}

	respItems := make([]*pb.TodayExerciseHistoryItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, convertTodayExerciseHistoryItem(item))
	}

	return &pb.ListTodayExerciseHistoryResponse{Items: respItems}, nil
}

// SaveCustomExercise 保存一条用户自定义动作。
func (a *ExerciseSetRecordApi) SaveCustomExercise(ctx context.Context, req *pb.SaveCustomExerciseRequest) (*pb.SaveCustomExerciseResponse, error) {
	uid := session.GetUser(ctx).UID()
	exercise := req.GetExercise()

	vipStatus, err := mysqlmodel.GetCurrentVIPStatus(uid, time.Now())
	if err != nil {
		return session.Error(ctx, gamecode.VIPStatusQueryFailed, &pb.SaveCustomExerciseResponse{})
	}
	if !vipStatus.IsVIP {
		return session.Error(ctx, gamecode.CustomExerciseVIPRequired, &pb.SaveCustomExerciseResponse{})
	}

	localID := strings.TrimSpace(exercise.GetLocalId())
	name := strings.TrimSpace(exercise.GetName())
	categoryKey := strings.TrimSpace(exercise.GetCategoryKey())
	subcategoryKey := strings.TrimSpace(exercise.GetSubcategoryKey())
	typeKey := strings.TrimSpace(exercise.GetTypeKey())

	if localID == "" {
		return session.Error(ctx, gamecode.CustomExerciseLocalIDEmpty, &pb.SaveCustomExerciseResponse{})
	}
	if name == "" {
		return session.Error(ctx, gamecode.CustomExerciseNameEmpty, &pb.SaveCustomExerciseResponse{})
	}
	if categoryKey == "" {
		return session.Error(ctx, gamecode.CustomExerciseCategoryEmpty, &pb.SaveCustomExerciseResponse{})
	}
	if typeKey == "" {
		return session.Error(ctx, gamecode.CustomExerciseTypeEmpty, &pb.SaveCustomExerciseResponse{})
	}

	saved, err := mysqlmodel.SaveCustomExercise(&mysqlmodel.CustomExercise{
		UID:             uid,
		LocalID:         localID,
		Name:            name,
		CategoryKey:     categoryKey,
		SubcategoryKey:  subcategoryKey,
		TypeKey:         typeKey,
		ClientCreatedAt: exercise.GetCreatedAt(),
	})
	if err != nil {
		return session.Error(ctx, gamecode.CustomExerciseSaveFailed, &pb.SaveCustomExerciseResponse{})
	}

	return &pb.SaveCustomExerciseResponse{
		Exercise: mysqlmodel.CustomExerciseToPB(saved),
	}, nil
}

// ListCustomExercises 查询当前用户的自定义动作。
func (a *ExerciseSetRecordApi) ListCustomExercises(ctx context.Context, req *pb.ListCustomExercisesRequest) (*pb.ListCustomExercisesResponse, error) {
	uid := session.GetUser(ctx).UID()

	exercises, err := mysqlmodel.ListCustomExercises(uid)
	if err != nil {
		return session.Error(ctx, gamecode.CustomExerciseListFailed, &pb.ListCustomExercisesResponse{})
	}

	respExercises := make([]*pb.CustomExercise, 0, len(exercises))
	for _, exercise := range exercises {
		respExercises = append(respExercises, mysqlmodel.CustomExerciseToPB(exercise))
	}

	return &pb.ListCustomExercisesResponse{Exercises: respExercises}, nil
}

// SaveExerciseTrainingSessionEndMarker 保存一条动作库训练手动结束标记。
func (a *ExerciseSetRecordApi) SaveExerciseTrainingSessionEndMarker(ctx context.Context, req *pb.SaveExerciseTrainingSessionEndMarkerRequest) (*pb.SaveExerciseTrainingSessionEndMarkerResponse, error) {
	uid := session.GetUser(ctx).UID()

	clientMarkerID := strings.TrimSpace(req.GetClientMarkerId())
	if clientMarkerID == "" {
		return session.Error(ctx, gamecode.ExerciseSessionEndMarkerClientIDEmpty, &pb.SaveExerciseTrainingSessionEndMarkerResponse{})
	}
	if req.GetEndedAt() <= 0 {
		return session.Error(ctx, gamecode.ExerciseSessionEndMarkerEndedAtInvalid, &pb.SaveExerciseTrainingSessionEndMarkerResponse{})
	}

	marker, err := mysqlmodel.SaveExerciseTrainingSessionEndMarker(&mysqlmodel.ExerciseTrainingSessionEndMarker{
		UID:            uid,
		ClientMarkerID: clientMarkerID,
		EndedAt:        req.GetEndedAt(),
	})
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseSessionEndMarkerSaveFailed, &pb.SaveExerciseTrainingSessionEndMarkerResponse{})
	}

	return &pb.SaveExerciseTrainingSessionEndMarkerResponse{
		Marker: mysqlmodel.ExerciseTrainingSessionEndMarkerToPB(marker),
	}, nil
}

// ListExerciseTrainingSessionEndMarkers 按时间范围查询动作库训练手动结束标记。
func (a *ExerciseSetRecordApi) ListExerciseTrainingSessionEndMarkers(ctx context.Context, req *pb.ListExerciseTrainingSessionEndMarkersRequest) (*pb.ListExerciseTrainingSessionEndMarkersResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetStartAt() <= 0 || req.GetEndAt() <= 0 || req.GetStartAt() > req.GetEndAt() {
		return session.Error(ctx, gamecode.ExerciseRecordTimeRangeInvalid, &pb.ListExerciseTrainingSessionEndMarkersResponse{})
	}

	markers, err := mysqlmodel.ListExerciseTrainingSessionEndMarkersByTimeRange(uid, req.GetStartAt(), req.GetEndAt())
	if err != nil {
		return session.Error(ctx, gamecode.ExerciseSessionEndMarkerListFailed, &pb.ListExerciseTrainingSessionEndMarkersResponse{})
	}

	respMarkers := make([]*pb.ExerciseTrainingSessionEndMarker, 0, len(markers))
	for _, marker := range markers {
		respMarkers = append(respMarkers, mysqlmodel.ExerciseTrainingSessionEndMarkerToPB(marker))
	}

	return &pb.ListExerciseTrainingSessionEndMarkersResponse{Markers: respMarkers}, nil
}

func validExerciseWeightUnit(unit pb.ExerciseWeightUnit) bool {
	switch unit {
	case pb.ExerciseWeightUnit_EXERCISE_WEIGHT_UNIT_JIN,
		pb.ExerciseWeightUnit_EXERCISE_WEIGHT_UNIT_KG,
		pb.ExerciseWeightUnit_EXERCISE_WEIGHT_UNIT_LB:
		return true
	default:
		return false
	}
}

func convertTodayExerciseHistoryItem(item *mysqlmodel.TodayExerciseHistory) *pb.TodayExerciseHistoryItem {
	if item == nil {
		return nil
	}

	latestRecords := make([]*pb.ExerciseSetRecord, 0, len(item.LatestRecords))
	for _, record := range item.LatestRecords {
		latestRecords = append(latestRecords, mysqlmodel.ExerciseRecordToPB(record))
	}

	return &pb.TodayExerciseHistoryItem{
		ExerciseId:           item.ExerciseID,
		ExerciseNameKey:      item.ExerciseNameKey,
		ExerciseNameSnapshot: item.ExerciseNameSnapshot,
		CategoryKey:          item.CategoryKey,
		TypeKey:              item.TypeKey,
		SetCount:             item.SetCount,
		MaxWeightX10:         item.MaxWeightX10,
		WeightUnit:           pb.ExerciseWeightUnit(item.WeightUnit),
		LatestRecordedAt:     item.LatestRecordedAt,
		LatestRecords:        latestRecords,
	}
}
