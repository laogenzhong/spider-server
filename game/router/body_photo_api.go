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

// BodyPhotoApi 实现照片索引相关 gRPC 接口。
type BodyPhotoApi struct {
	pb.UnimplementedBodyPhotoServiceServer
}

// SaveBodyPhoto 新增或更新照片索引。
func (a *BodyPhotoApi) SaveBodyPhoto(ctx context.Context, req *pb.SaveBodyPhotoRequest) (*pb.SaveBodyPhotoResponse, error) {
	uid := session.GetUser(ctx).UID()
	record := req.GetRecord()
	if record == nil {
		return session.Error(ctx, gamecode.BodyPhotoRecordEmpty, &pb.SaveBodyPhotoResponse{})
	}
	if record.GetClientRecordId() == "" {
		return session.Error(ctx, gamecode.BodyPhotoClientRecordIDEmpty, &pb.SaveBodyPhotoResponse{})
	}
	if record.GetPhotoLibraryAssetId() == "" {
		return session.Error(ctx, gamecode.BodyPhotoAssetIDEmpty, &pb.SaveBodyPhotoResponse{})
	}
	if record.GetKind() == pb.BodyPhotoKind_BODY_PHOTO_KIND_UNKNOWN {
		return session.Error(ctx, gamecode.BodyPhotoKindEmpty, &pb.SaveBodyPhotoResponse{})
	}
	if record.GetRecordAt() == 0 {
		return session.Error(ctx, gamecode.BodyPhotoRecordAtEmpty, &pb.SaveBodyPhotoResponse{})
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
		return session.Error(ctx, gamecode.BodyPhotoSaveFailed, &pb.SaveBodyPhotoResponse{})
	}

	return &pb.SaveBodyPhotoResponse{
		Record: convertBodyPhotoRecord(saved),
	}, nil
}

// DeleteBodyPhoto 删除照片索引。
func (a *BodyPhotoApi) DeleteBodyPhoto(ctx context.Context, req *pb.DeleteBodyPhotoRequest) (*pb.DeleteBodyPhotoResponse, error) {
	uid := session.GetUser(ctx).UID()
	if req.GetId() == 0 && req.GetClientRecordId() == "" {
		return session.Error(ctx, gamecode.BodyPhotoDeleteKeyEmpty, &pb.DeleteBodyPhotoResponse{})
	}

	err := mysqlmodel.DeleteBodyPhotoRecord(uid, req.GetId(), req.GetClientRecordId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return session.Error(ctx, gamecode.BodyPhotoNotFound, &pb.DeleteBodyPhotoResponse{})
	}
	if err != nil {
		return session.Error(ctx, gamecode.BodyPhotoDeleteFailed, &pb.DeleteBodyPhotoResponse{})
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
