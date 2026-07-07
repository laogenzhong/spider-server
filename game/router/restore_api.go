package router

import (
	"context"
	"strings"
	"time"

	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
)

const (
	defaultRestoreBatchSize uint32 = 200
	maxRestoreBatchSize     uint32 = 1000

	restoreTaskWeightRecords             = "weight_records"
	restoreTaskTrainingTags              = "training_tags"
	restoreTaskWorkoutTagBindings        = "workout_tag_bindings"
	restoreTaskBodyPhotos                = "body_photos"
	restoreTaskExerciseSetRecords        = "exercise_set_records"
	restoreTaskCustomExercises           = "custom_exercises"
	restoreTaskExerciseSessionEndMarkers = "exercise_training_session_end_markers"
	restoreTaskUserPreferences           = "user_preferences"
	restoreTaskWeeklyTrainingGoal        = "weekly_training_goal"
)

// ClientRestoreApi 实现客户端本地数据全量恢复和增量同步相关 gRPC 接口。
type ClientRestoreApi struct {
	pb.UnimplementedClientRestoreServiceServer
}

// GetRestorePlan 获取客户端数据同步计划。
func (a *ClientRestoreApi) GetRestorePlan(ctx context.Context, req *pb.RestorePlanRequest) (*pb.RestorePlanResponse, error) {
	uid := session.GetUser(ctx).UID()
	updateUserAppEnterFromRestorePlan(uid, req.GetSystemLanguage())

	startSnapshotID := req.GetStartSnapshotId()
	if startSnapshotID < 0 {
		return session.Error(ctx, gamecode.RestoreStartSnapshotInvalid, &pb.RestorePlanResponse{})
	}

	endSnapshotID := time.Now().UnixMilli()
	if startSnapshotID > endSnapshotID {
		return session.Error(ctx, gamecode.RestoreStartAfterCurrent, &pb.RestorePlanResponse{})
	}

	batchSize := normalizeRestoreBatchSize(req.GetPreferredBatchSize())
	tasks := make([]*pb.RestoreTask, 0, 6)
	var totalCount uint64

	weightCount, weightStartDate, weightEndDate, err := mysqlmodel.CountWeightRecordChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountWeightFailed, &pb.RestorePlanResponse{})
	}
	if weightCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskWeightRecords,
			pb.RestoreDataType_RESTORE_DATA_TYPE_WEIGHT_RECORDS,
			weightStartDate,
			weightEndDate,
			weightCount,
			batchSize,
		))
		totalCount += weightCount
	}

	tagCount, err := mysqlmodel.CountTrainingTagChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountTrainingTagsFailed, &pb.RestorePlanResponse{})
	}
	if tagCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskTrainingTags,
			pb.RestoreDataType_RESTORE_DATA_TYPE_TRAINING_TAGS,
			"",
			"",
			tagCount,
			batchSize,
		))
		totalCount += tagCount
	}

	bindingCount, bindingStartDate, bindingEndDate, err := mysqlmodel.CountWorkoutTagBindingChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountWorkoutTagBindingsFailed, &pb.RestorePlanResponse{})
	}
	if bindingCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskWorkoutTagBindings,
			pb.RestoreDataType_RESTORE_DATA_TYPE_WORKOUT_TAG_BINDINGS,
			bindingStartDate,
			bindingEndDate,
			bindingCount,
			batchSize,
		))
		totalCount += bindingCount
	}

	photoCount, photoStartDate, photoEndDate, err := mysqlmodel.CountBodyPhotoRecordChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountBodyPhotosFailed, &pb.RestorePlanResponse{})
	}
	if photoCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskBodyPhotos,
			pb.RestoreDataType_RESTORE_DATA_TYPE_BODY_PHOTOS,
			photoStartDate,
			photoEndDate,
			photoCount,
			batchSize,
		))
		totalCount += photoCount
	}

	exerciseCount, exerciseStartDate, exerciseEndDate, err := mysqlmodel.CountExerciseSetRecordChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountExerciseSetRecordsFailed, &pb.RestorePlanResponse{})
	}
	if exerciseCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskExerciseSetRecords,
			pb.RestoreDataType_RESTORE_DATA_TYPE_EXERCISE_SET_RECORDS,
			exerciseStartDate,
			exerciseEndDate,
			exerciseCount,
			batchSize,
		))
		totalCount += exerciseCount
	}

	customExerciseCount, _, _, err := mysqlmodel.CountCustomExerciseChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountCustomExercisesFailed, &pb.RestorePlanResponse{})
	}
	if customExerciseCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskCustomExercises,
			pb.RestoreDataType_RESTORE_DATA_TYPE_CUSTOM_EXERCISES,
			"",
			"",
			customExerciseCount,
			batchSize,
		))
		totalCount += customExerciseCount
	}

	exerciseSessionEndMarkerCount, markerStartDate, markerEndDate, err := mysqlmodel.CountExerciseTrainingSessionEndMarkerChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountExerciseSessionEndMarkersFailed, &pb.RestorePlanResponse{})
	}
	if exerciseSessionEndMarkerCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskExerciseSessionEndMarkers,
			pb.RestoreDataType_RESTORE_DATA_TYPE_EXERCISE_TRAINING_SESSION_END_MARKERS,
			markerStartDate,
			markerEndDate,
			exerciseSessionEndMarkerCount,
			batchSize,
		))
		totalCount += exerciseSessionEndMarkerCount
	}

	userPreferencesCount, err := mysqlmodel.CountUserPreferencesChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountUserPreferencesFailed, &pb.RestorePlanResponse{})
	}
	if userPreferencesCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskUserPreferences,
			pb.RestoreDataType_RESTORE_DATA_TYPE_USER_PREFERENCES,
			"",
			"",
			userPreferencesCount,
			batchSize,
		))
		totalCount += userPreferencesCount
	}

	weeklyTrainingGoalCount, err := mysqlmodel.CountWeeklyTrainingGoalChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return session.Error(ctx, gamecode.RestoreCountWeeklyTrainingGoalFailed, &pb.RestorePlanResponse{})
	}
	if weeklyTrainingGoalCount > 0 {
		tasks = append(tasks, buildRestoreTask(
			restoreTaskWeeklyTrainingGoal,
			pb.RestoreDataType_RESTORE_DATA_TYPE_WEEKLY_TRAINING_GOAL,
			"",
			"",
			weeklyTrainingGoalCount,
			batchSize,
		))
		totalCount += weeklyTrainingGoalCount
	}

	return &pb.RestorePlanResponse{
		StartSnapshotId: startSnapshotID,
		EndSnapshotId:   endSnapshotID,
		IsLatest:        totalCount == 0,
		Tasks:           tasks,
		TotalCount:      totalCount,
	}, nil
}

func updateUserAppEnterFromRestorePlan(uid uint64, systemLanguage string) {
	if uid == 0 {
		return
	}
	_ = mysqlmodel.UpdateUserLastAppEnter(uint(uid), time.Now(), normalizeRestoreSystemLanguage(systemLanguage))
}

func normalizeRestoreSystemLanguage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

// FetchRestoreBatch 按同步计划分批拉取数据。
func (a *ClientRestoreApi) FetchRestoreBatch(ctx context.Context, req *pb.RestoreBatchRequest) (*pb.RestoreBatchResponse, error) {
	uid := session.GetUser(ctx).UID()
	startSnapshotID := req.GetStartSnapshotId()
	endSnapshotID := req.GetEndSnapshotId()
	if startSnapshotID < 0 {
		return session.Error(ctx, gamecode.RestoreStartSnapshotInvalid, &pb.RestoreBatchResponse{})
	}
	if endSnapshotID <= 0 {
		return session.Error(ctx, gamecode.RestoreEndSnapshotEmpty, &pb.RestoreBatchResponse{})
	}
	if startSnapshotID > endSnapshotID {
		return session.Error(ctx, gamecode.RestoreStartAfterEnd, &pb.RestoreBatchResponse{})
	}
	if req.GetTaskId() == "" {
		return session.Error(ctx, gamecode.RestoreTaskIDEmpty, &pb.RestoreBatchResponse{})
	}

	batchSize := normalizeRestoreBatchSize(req.GetBatchSize())
	offset := int(req.GetBatchIndex() * batchSize)
	limit := int(batchSize)

	switch req.GetTaskId() {
	case restoreTaskWeightRecords:
		count, _, _, err := mysqlmodel.CountWeightRecordChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountWeightFailed, &pb.RestoreBatchResponse{})
		}
		records, err := mysqlmodel.ListWeightRecordChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchWeightFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.WeightRecordSyncItem, 0, len(records))
		for _, record := range records {
			items = append(items, &pb.WeightRecordSyncItem{
				Record:    convertWeightRecord(record),
				Deleted:   isDeletedInSnapshot(record.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(record.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(record.UpdatedAt, record.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_WEIGHT_RECORDS,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_WeightRecords{
			WeightRecords: &pb.WeightRecordRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskTrainingTags:
		count, err := mysqlmodel.CountTrainingTagChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountTrainingTagsFailed, &pb.RestoreBatchResponse{})
		}
		tags, err := mysqlmodel.ListTrainingTagChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchTrainingTagsFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.TrainingTagSyncItem, 0, len(tags))
		for _, tag := range tags {
			items = append(items, &pb.TrainingTagSyncItem{
				Tag:       convertTrainingTag(tag),
				Deleted:   isDeletedInSnapshot(tag.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(tag.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(tag.UpdatedAt, tag.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_TRAINING_TAGS,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_TrainingTags{
			TrainingTags: &pb.TrainingTagRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskWorkoutTagBindings:
		count, _, _, err := mysqlmodel.CountWorkoutTagBindingChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountWorkoutTagBindingsFailed, &pb.RestoreBatchResponse{})
		}
		bindings, err := mysqlmodel.ListWorkoutTagBindingChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchWorkoutTagBindingsFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.WorkoutTagBindingSyncItem, 0, len(bindings))
		for _, binding := range bindings {
			items = append(items, &pb.WorkoutTagBindingSyncItem{
				Binding:   convertWorkoutTagBinding(binding),
				Deleted:   isDeletedInSnapshot(binding.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(binding.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(binding.UpdatedAt, binding.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_WORKOUT_TAG_BINDINGS,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_WorkoutTagBindings{
			WorkoutTagBindings: &pb.WorkoutTagBindingRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskBodyPhotos:
		count, _, _, err := mysqlmodel.CountBodyPhotoRecordChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountBodyPhotosFailed, &pb.RestoreBatchResponse{})
		}
		records, err := mysqlmodel.ListBodyPhotoRecordChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchBodyPhotosFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.BodyPhotoSyncItem, 0, len(records))
		for _, record := range records {
			items = append(items, &pb.BodyPhotoSyncItem{
				Record:    convertBodyPhotoRecord(record),
				Deleted:   isDeletedInSnapshot(record.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(record.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(record.UpdatedAt, record.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_BODY_PHOTOS,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_BodyPhotos{
			BodyPhotos: &pb.BodyPhotoRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskExerciseSetRecords:
		count, _, _, err := mysqlmodel.CountExerciseSetRecordChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountExerciseSetRecordsFailed, &pb.RestoreBatchResponse{})
		}
		records, err := mysqlmodel.ListExerciseSetRecordChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchExerciseSetRecordsFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.ExerciseSetRecordSyncItem, 0, len(records))
		for _, record := range records {
			items = append(items, &pb.ExerciseSetRecordSyncItem{
				Record:    mysqlmodel.ExerciseRecordToPB(record),
				Deleted:   isDeletedInSnapshot(record.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(record.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(record.UpdatedAt, record.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_EXERCISE_SET_RECORDS,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_ExerciseSetRecords{
			ExerciseSetRecords: &pb.ExerciseSetRecordRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskCustomExercises:
		count, _, _, err := mysqlmodel.CountCustomExerciseChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountCustomExercisesFailed, &pb.RestoreBatchResponse{})
		}
		exercises, err := mysqlmodel.ListCustomExerciseChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchCustomExercisesFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.CustomExerciseSyncItem, 0, len(exercises))
		for _, exercise := range exercises {
			items = append(items, &pb.CustomExerciseSyncItem{
				Exercise:  mysqlmodel.CustomExerciseToPB(exercise),
				Deleted:   isDeletedInSnapshot(exercise.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(exercise.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(exercise.UpdatedAt, exercise.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_CUSTOM_EXERCISES,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_CustomExercises{
			CustomExercises: &pb.CustomExerciseRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskExerciseSessionEndMarkers:
		count, _, _, err := mysqlmodel.CountExerciseTrainingSessionEndMarkerChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountExerciseSessionEndMarkersFailed, &pb.RestoreBatchResponse{})
		}
		markers, err := mysqlmodel.ListExerciseTrainingSessionEndMarkerChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchExerciseSessionEndMarkersFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.ExerciseTrainingSessionEndMarkerSyncItem, 0, len(markers))
		for _, marker := range markers {
			items = append(items, &pb.ExerciseTrainingSessionEndMarkerSyncItem{
				Marker:    mysqlmodel.ExerciseTrainingSessionEndMarkerToPB(marker),
				Deleted:   isDeletedInSnapshot(marker.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(marker.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(marker.UpdatedAt, marker.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_EXERCISE_TRAINING_SESSION_END_MARKERS,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_ExerciseTrainingSessionEndMarkers{
			ExerciseTrainingSessionEndMarkers: &pb.ExerciseTrainingSessionEndMarkerRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskUserPreferences:
		count, err := mysqlmodel.CountUserPreferencesChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountUserPreferencesFailed, &pb.RestoreBatchResponse{})
		}
		records, err := mysqlmodel.ListUserPreferencesChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchUserPreferencesFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.UserPreferencesSyncItem, 0, len(records))
		for _, record := range records {
			items = append(items, &pb.UserPreferencesSyncItem{
				Preferences: mysqlmodel.UserPreferencesToPB(record),
				Deleted:     isDeletedInSnapshot(record.DeletedAt, endSnapshotID),
				DeletedAt:   deletedAtMillis(record.DeletedAt, endSnapshotID),
				ChangedAt:   changedAtMillis(record.UpdatedAt, record.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_USER_PREFERENCES,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_UserPreferences{
			UserPreferences: &pb.UserPreferencesRestoreBatch{Items: items},
		}
		return resp, nil

	case restoreTaskWeeklyTrainingGoal:
		count, err := mysqlmodel.CountWeeklyTrainingGoalChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreCountWeeklyTrainingGoalFailed, &pb.RestoreBatchResponse{})
		}
		records, err := mysqlmodel.ListWeeklyTrainingGoalChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return session.Error(ctx, gamecode.RestoreFetchWeeklyTrainingGoalFailed, &pb.RestoreBatchResponse{})
		}
		items := make([]*pb.WeeklyTrainingGoalSyncItem, 0, len(records))
		for _, record := range records {
			items = append(items, &pb.WeeklyTrainingGoalSyncItem{
				Goal:      mysqlmodel.WeeklyTrainingGoalToPB(record),
				Deleted:   isDeletedInSnapshot(record.DeletedAt, endSnapshotID),
				DeletedAt: deletedAtMillis(record.DeletedAt, endSnapshotID),
				ChangedAt: changedAtMillis(record.UpdatedAt, record.DeletedAt, endSnapshotID),
			})
		}

		resp := buildRestoreBatchResponse(
			pb.RestoreDataType_RESTORE_DATA_TYPE_WEEKLY_TRAINING_GOAL,
			req.GetBatchIndex(),
			uint32(len(items)),
			count,
			batchSize,
			startSnapshotID,
			endSnapshotID,
		)
		resp.Payload = &pb.RestoreBatchResponse_WeeklyTrainingGoal{
			WeeklyTrainingGoal: &pb.WeeklyTrainingGoalRestoreBatch{Items: items},
		}
		return resp, nil
	}

	return session.Error(ctx, gamecode.RestoreTaskUnknown, &pb.RestoreBatchResponse{})
}

func buildRestoreTask(taskID string, dataType pb.RestoreDataType, startDate string, endDate string, totalCount uint64, batchSize uint32) *pb.RestoreTask {
	return &pb.RestoreTask{
		TaskId:          taskID,
		DataType:        dataType,
		StartDate:       startDate,
		EndDate:         endDate,
		TotalCount:      totalCount,
		BatchSize:       batchSize,
		TotalBatches:    restoreTotalBatches(totalCount, batchSize),
		StartBatchIndex: 0,
	}
}

func buildRestoreBatchResponse(dataType pb.RestoreDataType, batchIndex uint32, batchCount uint32, totalCount uint64, batchSize uint32, startSnapshotID int64, endSnapshotID int64) *pb.RestoreBatchResponse {
	totalBatches := restoreTotalBatches(totalCount, batchSize)
	nextBatchIndex := batchIndex + 1
	hasMore := nextBatchIndex < totalBatches
	if !hasMore {
		nextBatchIndex = 0
	}

	return &pb.RestoreBatchResponse{
		DataType:        dataType,
		BatchIndex:      batchIndex,
		BatchCount:      batchCount,
		TotalCount:      totalCount,
		TotalBatches:    totalBatches,
		HasMore:         hasMore,
		NextBatchIndex:  nextBatchIndex,
		StartSnapshotId: startSnapshotID,
		EndSnapshotId:   endSnapshotID,
	}
}

func normalizeRestoreBatchSize(batchSize uint32) uint32 {
	if batchSize == 0 {
		return defaultRestoreBatchSize
	}
	if batchSize > maxRestoreBatchSize {
		return maxRestoreBatchSize
	}
	return batchSize
}

func restoreTotalBatches(totalCount uint64, batchSize uint32) uint32 {
	if totalCount == 0 || batchSize == 0 {
		return 0
	}
	return uint32((totalCount + uint64(batchSize) - 1) / uint64(batchSize))
}
