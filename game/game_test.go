package game

import (
	"context"
	"testing"
	"time"

	pb "spider-server/gen/spider/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRoomSyncApiDirectGRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(
		ctx,
		"localhost:18000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc test server failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewRoomSyncApiClient(conn)

	resp, err := client.Sync(ctx, &pb.SyncRequest{})
	if err != nil {
		t.Fatalf("call RoomSyncApi.Sync failed: %v", err)
	}

	if resp == nil {
		t.Fatal("sync response is nil")
	}

	if resp.Time == 0 {
		t.Fatalf("unexpected sync response time: got %d", resp.Time)
	}
}
