package router

import (
	"context"
	"errors"

	"spider-server/game/session"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// BodyPhotoApi 实现照片索引相关 gRPC 接口。
type BodyPhotoApi struct {
	pb.UnimplementedBodyPhotoServiceServer
}

// SaveBodyPhoto 新增或更新照片索引。
func (a *BodyPhotoApi) SaveBodyPhoto(ctx context.Context, req *pb.SaveBodyPhotoRequest) (*pb.SaveBodyPhotoResponse, error) {
	uid := session.GetUser(ctx).UID()
	record := req.GetRecord()
	if record == nil {
		return nil, status.Error(codes.InvalidArgument, "record 不能为空")
	}
	if record.GetClientRecordId() == "" {
		return nil, status.Error(codes.InvalidArgument, "client_record_id 不能为空")
	}
	if record.GetPhotoLibraryAssetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "photo_library_asset_id 不能为空")
	}
	if record.GetKind() == pb.BodyPhotoKind_BODY_PHOTO_KIND_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument, "kind 不能为空")
	}
	if record.GetRecordAt() == 0 {
		return nil, status.Error(codes.InvalidArgument, "record_at 不能为空")
	}

	saved, err := mysqlmodel.SaveBodyPhotoRecord(&mysqlmodel.BodyPhotoRecord{
		UID:                 uid,
		ClientRecordID:      record.GetClientRecordId(),
		PhotoLibraryAssetID: record.GetPhotoLibraryAssetId(),
		Kind:                int32(record.GetKind()),
		RecordAt:            record.GetRecordAt(),
		Weight:              record.GetWeight(),
		Note:                record.GetNote(),
		FileName:            record.GetFileName(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "保存照片索引失败：%v", err)
	}

	return &pb.SaveBodyPhotoResponse{
		Record: convertBodyPhotoRecord(saved),
	}, nil
}

// DeleteBodyPhoto 删除照片索引。
func (a *BodyPhotoApi) DeleteBodyPhoto(ctx context.Context, req *pb.DeleteBodyPhotoRequest) (*pb.DeleteBodyPhotoResponse, error) {
	uid := session.GetUser(ctx).UID()
	if req.GetId() == 0 && req.GetClientRecordId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id 和 client_record_id 不能同时为空")
	}

	err := mysqlmodel.DeleteBodyPhotoRecord(uid, req.GetId(), req.GetClientRecordId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "照片索引不存在")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "删除照片索引失败：%v", err)
	}

	return &pb.DeleteBodyPhotoResponse{Success: true}, nil
}

// convertBodyPhotoRecord 将 MySQL 照片索引模型转换为 pb 模型。
func convertBodyPhotoRecord(record *mysqlmodel.BodyPhotoRecord) *pb.BodyPhotoRecord {
	if record == nil {
		return nil
	}

	return &pb.BodyPhotoRecord{
		Id:                  record.ID,
		Uid:                 record.UID,
		ClientRecordId:      record.ClientRecordID,
		PhotoLibraryAssetId: record.PhotoLibraryAssetID,
		Kind:                pb.BodyPhotoKind(record.Kind),
		RecordAt:            record.RecordAt,
		Weight:              record.Weight,
		Note:                record.Note,
		FileName:            record.FileName,
		CreatedAt:           millis(record.CreatedAt),
		UpdatedAt:           millis(record.UpdatedAt),
	}
}
