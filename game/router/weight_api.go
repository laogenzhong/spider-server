package router

import (
	"context"
	"errors"
	gamecode "spider-server/game/code"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
	"time"
)

// WeightApi 实现体重记录相关 gRPC 接口。
//
// 设计原则：
// 1. 服务端 MySQL 是体重记录的主数据源。
// 2. 客户端新增、修改、删除都以服务端成功为准。
// 3. uid 从登录 token/session 对应的 ctx 中获取，客户端不需要主动传 uid。
// 4. 体重 weight 使用 kg * 10 的整数，例如 70.5kg 存为 705。
// 5. satiety 是摄入状态评分，范围 0-10，不是具体 kcal。
type WeightApi struct {
	pb.UnimplementedWeightRecordServiceServer
}

// NewWeightApi 创建体重记录服务实例。
func NewWeightApi() *WeightApi {
	return &WeightApi{}
}

// SaveWeightRecord 保存某一天的体重记录。
//
// 按 uid + record_date 判断：当天没有记录就新增，当天已有记录就修改。
func (a *WeightApi) SaveWeightRecord(ctx context.Context, req *pb.SaveWeightRecordRequest) (*pb.SaveWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetRecordDate() == "" {
		return session.Error(ctx, gamecode.WeightRecordDateEmpty, &pb.SaveWeightRecordResponse{})
	}
	if req.GetWeight() <= 0 {
		return session.Error(ctx, gamecode.WeightValueInvalid, &pb.SaveWeightRecordResponse{})
	}
	if req.GetSatiety() < 0 || req.GetSatiety() > 10 {
		return session.Error(ctx, gamecode.WeightSatietyInvalid, &pb.SaveWeightRecordResponse{})
	}

	record, err := mysqlmodel.CreateWeightRecord(uid, req.GetRecordDate(), req.GetWeight(), req.GetSatiety())
	if errors.Is(err, mysqlmodel.ErrWeightRecordDailyCreateLimitExceeded) {
		return session.Error(ctx, gamecode.WeightDailyCreateLimitExceeded, &pb.SaveWeightRecordResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.WeightSaveFailed, &pb.SaveWeightRecordResponse{})
	}

	return &pb.SaveWeightRecordResponse{
		Record: convertWeightRecord(record),
	}, nil
}

// DeleteWeightRecord 删除体重记录。
//
// 优先使用 id 删除；如果 id 为空，则使用 record_date 删除当前用户当天记录。
func (a *WeightApi) DeleteWeightRecord(ctx context.Context, req *pb.DeleteWeightRecordRequest) (*pb.DeleteWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 && req.GetRecordDate() == "" {
		return session.Error(ctx, gamecode.WeightDeleteKeyEmpty, &pb.DeleteWeightRecordResponse{})
	}

	if err := mysqlmodel.DeleteWeightRecord(uid, req.GetId(), req.GetRecordDate()); err != nil {
		return session.Error(ctx, gamecode.WeightDeleteFailed, &pb.DeleteWeightRecordResponse{})
	}

	return &pb.DeleteWeightRecordResponse{Success: true}, nil
}

// GetWeightRecord 查询某一天的体重记录。
func (a *WeightApi) GetWeightRecord(ctx context.Context, req *pb.GetWeightRecordRequest) (*pb.GetWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetRecordDate() == "" {
		return session.Error(ctx, gamecode.WeightRecordDateEmpty, &pb.GetWeightRecordResponse{})
	}

	record, err := mysqlmodel.GetWeightRecordByDate(uid, req.GetRecordDate())
	if err != nil {
		if err.Error() == "record not found" {
			return &pb.GetWeightRecordResponse{Exists: false}, nil
		}
		return session.Error(ctx, gamecode.WeightQueryFailed, &pb.GetWeightRecordResponse{})
	}

	return &pb.GetWeightRecordResponse{
		Exists: true,
		Record: convertWeightRecord(record),
	}, nil
}

// ListWeightRecords 查询日期范围内的体重记录。
func (a *WeightApi) ListWeightRecords(ctx context.Context, req *pb.ListWeightRecordsRequest) (*pb.ListWeightRecordsResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetStartDate() == "" || req.GetEndDate() == "" {
		return session.Error(ctx, gamecode.WeightDateRangeEmpty, &pb.ListWeightRecordsResponse{})
	}

	records, err := mysqlmodel.ListWeightRecords(uid, req.GetStartDate(), req.GetEndDate())
	if err != nil {
		return session.Error(ctx, gamecode.WeightListFailed, &pb.ListWeightRecordsResponse{})
	}

	respRecords := make([]*pb.WeightRecord, 0, len(records))
	for _, record := range records {
		respRecords = append(respRecords, convertWeightRecord(record))
	}

	return &pb.ListWeightRecordsResponse{Records: respRecords}, nil
}

// GetLatestWeightRecord 获取当前用户最近一条体重记录。
func (a *WeightApi) GetLatestWeightRecord(ctx context.Context, req *pb.GetLatestWeightRecordRequest) (*pb.GetLatestWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	record, err := mysqlmodel.GetLatestWeightRecord(uid)
	if err != nil {
		if err.Error() == "record not found" {
			return &pb.GetLatestWeightRecordResponse{Exists: false}, nil
		}
		return session.Error(ctx, gamecode.WeightLatestQueryFailed, &pb.GetLatestWeightRecordResponse{})
	}

	return &pb.GetLatestWeightRecordResponse{
		Exists: true,
		Record: convertWeightRecord(record),
	}, nil
}

// convertWeightRecord 将 MySQL 体重记录模型转换为 pb 模型。
func convertWeightRecord(record *mysqlmodel.WeightRecord) *pb.WeightRecord {
	if record == nil {
		return nil
	}

	return &pb.WeightRecord{
		Id:         record.ID,
		Uid:        record.UID,
		RecordDate: record.RecordDate,
		Weight:     record.Weight,
		Satiety:    record.Satiety,
		CreatedAt:  millis(record.CreatedAt),
		UpdatedAt:  millis(record.UpdatedAt),
	}
}

// millis 将 time.Time 转为毫秒时间戳。
func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
