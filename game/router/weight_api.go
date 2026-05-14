package router

import (
	"context"
	"errors"
	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
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

// CreateWeightRecord 新增某一天的体重记录。
func (a *WeightApi) CreateWeightRecord(ctx context.Context, req *pb.CreateWeightRecordRequest) (*pb.CreateWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetRecordDate() == "" {
		return nil, status.Error(codes.InvalidArgument, "record_date 不能为空")
	}
	if req.GetWeight() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "weight 必须大于 0")
	}
	if req.GetSatiety() < 0 || req.GetSatiety() > 10 {
		return nil, status.Error(codes.InvalidArgument, "satiety 必须在 0-10 之间")
	}

	record, err := mysqlmodel.CreateWeightRecord(uid, req.GetRecordDate(), req.GetWeight(), req.GetSatiety())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "创建体重记录失败：%v", err)
	}

	return &pb.CreateWeightRecordResponse{
		Record: convertWeightRecord(record),
	}, nil
}

// UpdateWeightRecord 修改体重记录。
//
// 优先使用 id 定位记录；如果 id 为空，则使用 record_date 定位当前用户当天记录。
func (a *WeightApi) UpdateWeightRecord(ctx context.Context, req *pb.UpdateWeightRecordRequest) (*pb.UpdateWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 && req.GetRecordDate() == "" {
		return nil, status.Error(codes.InvalidArgument, "id 和 record_date 不能同时为空")
	}
	if req.GetWeight() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "weight 必须大于 0")
	}
	if req.GetSatiety() < 0 || req.GetSatiety() > 10 {
		return nil, status.Error(codes.InvalidArgument, "satiety 必须在 0-10 之间")
	}

	record, err := mysqlmodel.UpdateWeightRecord(uid, req.GetId(), req.GetRecordDate(), req.GetWeight(), req.GetSatiety())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "体重记录不存在")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "修改体重记录失败：%v", err)
	}

	return &pb.UpdateWeightRecordResponse{
		Record: convertWeightRecord(record),
	}, nil
}

// DeleteWeightRecord 删除体重记录。
//
// 优先使用 id 删除；如果 id 为空，则使用 record_date 删除当前用户当天记录。
func (a *WeightApi) DeleteWeightRecord(ctx context.Context, req *pb.DeleteWeightRecordRequest) (*pb.DeleteWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetId() == 0 && req.GetRecordDate() == "" {
		return nil, status.Error(codes.InvalidArgument, "id 和 record_date 不能同时为空")
	}

	if err := mysqlmodel.DeleteWeightRecord(uid, req.GetId(), req.GetRecordDate()); err != nil {
		return nil, status.Errorf(codes.Internal, "删除体重记录失败：%v", err)
	}

	return &pb.DeleteWeightRecordResponse{Success: true}, nil
}

// GetWeightRecord 查询某一天的体重记录。
func (a *WeightApi) GetWeightRecord(ctx context.Context, req *pb.GetWeightRecordRequest) (*pb.GetWeightRecordResponse, error) {
	uid := session.GetUser(ctx).UID()

	if req.GetRecordDate() == "" {
		return nil, status.Error(codes.InvalidArgument, "record_date 不能为空")
	}

	record, err := mysqlmodel.GetWeightRecordByDate(uid, req.GetRecordDate())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &pb.GetWeightRecordResponse{Exists: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "查询体重记录失败：%v", err)
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
		return nil, status.Error(codes.InvalidArgument, "start_date 和 end_date 不能为空")
	}

	records, err := mysqlmodel.ListWeightRecords(uid, req.GetStartDate(), req.GetEndDate())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "查询体重记录列表失败：%v", err)
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
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &pb.GetLatestWeightRecordResponse{Exists: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "查询最近体重记录失败：%v", err)
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
