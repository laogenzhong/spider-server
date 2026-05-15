package router

import (
	"context"
	"time"

	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	defaultRestoreBatchSize uint32 = 200
	maxRestoreBatchSize     uint32 = 1000

	restoreTaskWeightRecords      = "weight_records"
	restoreTaskTrainingTags       = "training_tags"
	restoreTaskWorkoutTagBindings = "workout_tag_bindings"
)

// ClientRestoreApi 实现客户端本地数据全量恢复和增量同步相关 gRPC 接口。
type ClientRestoreApi struct {
	pb.UnimplementedClientRestoreServiceServer
}

// GetRestorePlan 获取客户端数据同步计划。
func (a *ClientRestoreApi) GetRestorePlan(ctx context.Context, req *pb.RestorePlanRequest) (*pb.RestorePlanResponse, error) {
	uid := session.GetUser(ctx).UID()
	startSnapshotID := req.GetStartSnapshotId()
	if startSnapshotID < 0 {
		return nil, status.Error(codes.InvalidArgument, "start_snapshot_id 不能小于 0")
	}

	endSnapshotID := time.Now().UnixMilli()
	if startSnapshotID > endSnapshotID {
		return nil, status.Error(codes.InvalidArgument, "start_snapshot_id 不能大于当前服务端快照")
	}

	batchSize := normalizeRestoreBatchSize(req.GetPreferredBatchSize())
	tasks := make([]*pb.RestoreTask, 0, 3)
	var totalCount uint64

	weightCount, weightStartDate, weightEndDate, err := mysqlmodel.CountWeightRecordChanges(uid, startSnapshotID, endSnapshotID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "统计体重同步数据失败：%v", err)
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
		return nil, status.Errorf(codes.Internal, "统计训练标签同步数据失败：%v", err)
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
		return nil, status.Errorf(codes.Internal, "统计训练标签绑定同步数据失败：%v", err)
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

	return &pb.RestorePlanResponse{
		StartSnapshotId: startSnapshotID,
		EndSnapshotId:   endSnapshotID,
		IsLatest:        totalCount == 0,
		Tasks:           tasks,
		TotalCount:      totalCount,
	}, nil
}

// FetchRestoreBatch 按同步计划分批拉取数据。
func (a *ClientRestoreApi) FetchRestoreBatch(ctx context.Context, req *pb.RestoreBatchRequest) (*pb.RestoreBatchResponse, error) {
	uid := session.GetUser(ctx).UID()
	startSnapshotID := req.GetStartSnapshotId()
	endSnapshotID := req.GetEndSnapshotId()
	if startSnapshotID < 0 {
		return nil, status.Error(codes.InvalidArgument, "start_snapshot_id 不能小于 0")
	}
	if endSnapshotID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "end_snapshot_id 不能为空")
	}
	if startSnapshotID > endSnapshotID {
		return nil, status.Error(codes.InvalidArgument, "start_snapshot_id 不能大于 end_snapshot_id")
	}
	if req.GetTaskId() == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id 不能为空")
	}

	batchSize := normalizeRestoreBatchSize(req.GetBatchSize())
	offset := int(req.GetBatchIndex() * batchSize)
	limit := int(batchSize)

	switch req.GetTaskId() {
	case restoreTaskWeightRecords:
		count, _, _, err := mysqlmodel.CountWeightRecordChanges(uid, startSnapshotID, endSnapshotID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "统计体重同步数据失败：%v", err)
		}
		records, err := mysqlmodel.ListWeightRecordChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "拉取体重同步数据失败：%v", err)
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
			return nil, status.Errorf(codes.Internal, "统计训练标签同步数据失败：%v", err)
		}
		tags, err := mysqlmodel.ListTrainingTagChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "拉取训练标签同步数据失败：%v", err)
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
			return nil, status.Errorf(codes.Internal, "统计训练标签绑定同步数据失败：%v", err)
		}
		bindings, err := mysqlmodel.ListWorkoutTagBindingChangesPage(uid, startSnapshotID, endSnapshotID, limit, offset)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "拉取训练标签绑定同步数据失败：%v", err)
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
	}

	return nil, status.Errorf(codes.InvalidArgument, "未知同步任务：%s", req.GetTaskId())
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

func isDeletedInSnapshot(deletedAt gorm.DeletedAt, endSnapshotID int64) bool {
	return deletedAt.Valid && deletedAt.Time.UnixMilli() <= endSnapshotID
}

func deletedAtMillis(deletedAt gorm.DeletedAt, endSnapshotID int64) int64 {
	if !isDeletedInSnapshot(deletedAt, endSnapshotID) {
		return 0
	}
	return deletedAt.Time.UnixMilli()
}

func changedAtMillis(updatedAt time.Time, deletedAt gorm.DeletedAt, endSnapshotID int64) int64 {
	changedAt := updatedAt.UnixMilli()
	if isDeletedInSnapshot(deletedAt, endSnapshotID) && deletedAt.Time.UnixMilli() > changedAt {
		return deletedAt.Time.UnixMilli()
	}
	return changedAt
}
