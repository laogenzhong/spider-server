package router

import (
	"context"
	"errors"
	"strings"
	"time"

	appconfig "spider-server/common/config"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/protobuf/proto"
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

// SyncWorkoutDataSnapshots 批量保存动作库、计划文件夹或整次训练快照。
// 客户端以 client_snapshot_id 和 kind/entity_id 保证重试幂等。
func (a *ExerciseSetRecordApi) SyncWorkoutDataSnapshots(ctx context.Context, req *pb.SyncWorkoutDataSnapshotsRequest) (*pb.SyncWorkoutDataSnapshotsResponse, error) {
	uid := session.GetUser(ctx).UID()
	if int64(proto.Size(req)) > maxWorkoutDataSyncRequestBytes {
		return session.Error(ctx, gamecode.WorkoutDataSnapshotsRequestTooLarge, &pb.SyncWorkoutDataSnapshotsResponse{})
	}
	snapshots := req.GetSnapshots()
	if len(snapshots) == 0 || len(snapshots) > 20 {
		return session.Error(ctx, gamecode.WorkoutDataSnapshotsEmpty, &pb.SyncWorkoutDataSnapshotsResponse{})
	}
	for _, snapshot := range snapshots {
		if !validWorkoutDataSnapshot(snapshot) {
			return session.Error(ctx, gamecode.WorkoutDataSnapshotInvalid, &pb.SyncWorkoutDataSnapshotsResponse{})
		}
	}
	accepted, err := mysqlmodel.SaveWorkoutDataSnapshots(uid, snapshots)
	if err != nil {
		return session.Error(ctx, gamecode.WorkoutDataSnapshotSaveFailed, &pb.SyncWorkoutDataSnapshotsResponse{})
	}
	return &pb.SyncWorkoutDataSnapshotsResponse{AcceptedClientSnapshotIds: accepted}, nil
}

func validWorkoutDataSnapshot(snapshot *pb.WorkoutDataSnapshot) bool {
	if snapshot == nil || strings.TrimSpace(snapshot.GetClientSnapshotId()) == "" || strings.TrimSpace(snapshot.GetEntityId()) == "" || snapshot.GetChangedAt() <= 0 {
		return false
	}
	if len(snapshot.GetClientSnapshotId()) > 64 || len(snapshot.GetEntityId()) > 64 {
		return false
	}
	if snapshot.GetDeleted() {
		return snapshot.GetKind() == pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN_FOLDER ||
			snapshot.GetKind() == pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN
	}
	switch snapshot.GetKind() {
	case pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_LIBRARY:
		library := snapshot.GetLibrary()
		return snapshot.Library != nil && validWorkoutLibrarySnapshot(library)
	case pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_LIBRARY_METADATA:
		library := snapshot.GetLibrary()
		return snapshot.Library != nil && len(library.GetFolders()) == 0 && validCustomExerciseSnapshots(library.GetCustomExercises()) && validUniqueSnapshotIDs(library.GetFolderIds(), maxWorkoutPlanItemsPerLevel)
	case pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN_FOLDER:
		folder := snapshot.GetPlanFolder()
		return snapshot.PlanFolder != nil && folder.GetId() == snapshot.GetEntityId() && validWorkoutPlanFolderEntitySnapshot(folder)
	case pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_PLAN:
		entity := snapshot.GetPlanEntity()
		return snapshot.PlanEntity != nil && validWorkoutPlanEntitySnapshot(snapshot.GetEntityId(), entity)
	case pb.WorkoutDataSnapshotKind_WORKOUT_DATA_SNAPSHOT_KIND_TRAINING_SESSION:
		training := snapshot.GetTrainingSession()
		if snapshot.TrainingSession == nil || !validRequiredString(training.GetSessionId(), maxWorkoutSnapshotIDBytes) || training.GetSessionId() != snapshot.GetEntityId() || !validOptionalString(training.GetPlanId(), maxWorkoutSnapshotIDBytes) || !validOptionalString(training.GetPlanTitle(), maxWorkoutSnapshotTitleBytes) || training.GetStartedAt() <= 0 || training.GetEndedAt() < training.GetStartedAt() || len(training.GetRecords()) > 1000 || len(training.GetRecordLinks()) > 1000 {
			return false
		}
		recordCountsByExercise := make(map[string]int)
		for _, record := range training.GetRecords() {
			if record == nil || !validRequiredString(record.GetClientRecordId(), maxWorkoutSnapshotIDBytes) || !validRequiredString(record.GetExerciseId(), maxWorkoutSnapshotIDBytes) || !validOptionalString(record.GetExerciseNameKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(record.GetExerciseNameSnapshot(), maxWorkoutSnapshotNameBytes) || !validOptionalString(record.GetCategoryKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(record.GetTypeKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(record.GetSessionId(), maxWorkoutSnapshotIDBytes) || (record.GetSessionId() != "" && record.GetSessionId() != training.GetSessionId()) || record.GetWeightX10() < 0 || record.GetReps() <= 0 || !validExerciseWeightUnit(record.GetWeightUnit()) {
				return false
			}
			exerciseID := strings.TrimSpace(record.GetExerciseId())
			recordCountsByExercise[exerciseID]++
			if len(recordCountsByExercise) > maxWorkoutPlanItemsPerLevel || recordCountsByExercise[exerciseID] > maxWorkoutPlanItemsPerLevel {
				return false
			}
		}
		return true
	default:
		return false
	}
}

const (
	maxWorkoutPlanItemsPerLevel    = 99
	maxWorkoutSnapshotIDBytes      = 64
	maxWorkoutSnapshotTitleBytes   = 160
	maxWorkoutSnapshotKeyBytes     = 128
	maxWorkoutSnapshotNameBytes    = 256
	maxWorkoutSnapshotNoteBytes    = 2048
	maxWorkoutSnapshotSetTextBytes = 32
)

var maxWorkoutDataSyncRequestBytes = appconfig.Default().WorkoutDataSync.SyncRPCMaxRequestBytes

func ConfigureWorkoutDataSyncLimits(cfg appconfig.WorkoutDataSyncConfig) {
	if cfg.SyncRPCMaxRequestBytes <= 0 {
		cfg.SyncRPCMaxRequestBytes = appconfig.Default().WorkoutDataSync.SyncRPCMaxRequestBytes
	}
	maxWorkoutDataSyncRequestBytes = cfg.SyncRPCMaxRequestBytes
	mysqlmodel.ConfigureWorkoutDataSnapshotLimits(cfg)
}

func validWorkoutLibrarySnapshot(library *pb.WorkoutLibrarySnapshot) bool {
	if library == nil || len(library.GetFolders()) > maxWorkoutPlanItemsPerLevel || !validCustomExerciseSnapshots(library.GetCustomExercises()) || !validUniqueSnapshotIDs(library.GetFolderIds(), maxWorkoutPlanItemsPerLevel) {
		return false
	}
	for _, folder := range library.GetFolders() {
		if folder == nil || !validRequiredString(folder.GetId(), maxWorkoutSnapshotIDBytes) || !validRequiredString(folder.GetTitle(), maxWorkoutSnapshotTitleBytes) || len(folder.GetPlans()) > maxWorkoutPlanItemsPerLevel {
			return false
		}
		for _, plan := range folder.GetPlans() {
			if !validWorkoutPlanSnapshot(plan) {
				return false
			}
		}
	}
	return true
}

func validWorkoutPlanFolderEntitySnapshot(folder *pb.WorkoutPlanFolderEntitySnapshot) bool {
	if folder == nil || !validRequiredString(folder.GetId(), maxWorkoutSnapshotIDBytes) || !validRequiredString(folder.GetTitle(), maxWorkoutSnapshotTitleBytes) || folder.GetSortIndex() < 0 || folder.GetSortIndex() >= maxWorkoutPlanItemsPerLevel || !validUniqueSnapshotIDs(folder.GetPlanIds(), maxWorkoutPlanItemsPerLevel) {
		return false
	}
	return true
}

func validWorkoutPlanEntitySnapshot(entityID string, entity *pb.WorkoutPlanEntitySnapshot) bool {
	return entity != nil && validRequiredString(entity.GetFolderId(), maxWorkoutSnapshotIDBytes) && entity.GetSortIndex() >= 0 && entity.GetSortIndex() < maxWorkoutPlanItemsPerLevel && entity.GetPlan() != nil && entity.GetPlan().GetId() == entityID && validWorkoutPlanSnapshot(entity.GetPlan())
}

func validWorkoutPlanSnapshot(plan *pb.WorkoutPlanSnapshot) bool {
	if plan == nil || !validRequiredString(plan.GetId(), maxWorkoutSnapshotIDBytes) || !validRequiredString(plan.GetTitle(), maxWorkoutSnapshotTitleBytes) || len(plan.GetExercises()) > maxWorkoutPlanItemsPerLevel {
		return false
	}
	for _, exercise := range plan.GetExercises() {
		if exercise == nil || !validRequiredString(exercise.GetId(), maxWorkoutSnapshotIDBytes) || !validRequiredString(exercise.GetExerciseId(), maxWorkoutSnapshotIDBytes) || !validOptionalString(exercise.GetNameKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(exercise.GetNameSnapshot(), maxWorkoutSnapshotNameBytes) || !validOptionalString(exercise.GetCategoryKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(exercise.GetTypeKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(exercise.GetDisplayTypeKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(exercise.GetCustomName(), maxWorkoutSnapshotNameBytes) || !validOptionalString(exercise.GetCustomSubcategoryKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(exercise.GetNote(), maxWorkoutSnapshotNoteBytes) || !validOptionalString(exercise.GetWeightUnit(), 8) || exercise.GetSetCount() <= 0 || exercise.GetSetCount() > maxWorkoutPlanItemsPerLevel || len(exercise.GetSets()) > maxWorkoutPlanItemsPerLevel {
			return false
		}
		for _, set := range exercise.GetSets() {
			if set == nil || !validRequiredString(set.GetId(), maxWorkoutSnapshotIDBytes) || !validOptionalString(set.GetWeightText(), maxWorkoutSnapshotSetTextBytes) || !validOptionalString(set.GetRepsText(), maxWorkoutSnapshotSetTextBytes) {
				return false
			}
		}
	}
	return true
}

func validCustomExerciseSnapshots(exercises []*pb.CustomExercise) bool {
	if len(exercises) > 500 {
		return false
	}
	for _, exercise := range exercises {
		if exercise == nil || !validRequiredString(exercise.GetLocalId(), maxWorkoutSnapshotIDBytes) || !validRequiredString(exercise.GetName(), maxWorkoutSnapshotNameBytes) || !validRequiredString(exercise.GetCategoryKey(), maxWorkoutSnapshotKeyBytes) || !validRequiredString(exercise.GetTypeKey(), maxWorkoutSnapshotKeyBytes) || !validOptionalString(exercise.GetSubcategoryKey(), maxWorkoutSnapshotKeyBytes) {
			return false
		}
	}
	return true
}

func validRequiredString(value string, maxBytes int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(value) <= maxBytes
}

func validOptionalString(value string, maxBytes int) bool {
	return len(value) <= maxBytes
}

func validUniqueSnapshotIDs(ids []string, maxCount int) bool {
	if len(ids) > maxCount {
		return false
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if !validRequiredString(id, maxWorkoutSnapshotIDBytes) {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
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
