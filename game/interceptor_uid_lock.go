package game

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	"spider-server/game/session"
)

const uidBusinessLockShardCount = 1024

var uidBusinessLocks [uidBusinessLockShardCount]sync.Mutex

func uidLockUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	user := session.GetUser(ctx)
	if user == nil || user.UIDOrDefault() == 0 {
		return handler(ctx, req)
	}

	unlock := lockUIDBusiness(user.UIDOrDefault())
	defer unlock()

	return handler(ctx, req)
}

func lockUIDBusiness(uid uint64) func() {
	lock := &uidBusinessLocks[uid%uidBusinessLockShardCount]
	lock.Lock()
	return lock.Unlock
}
