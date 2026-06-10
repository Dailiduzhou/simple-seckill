package biz

import "context"

type SeckillSagaArgs struct {
	GID       string
	UserID    int64
	ProductID int64
	Amount    int32
}

type SAGACoordinator interface {
	ExecuteSeckillSaga(ctx context.Context, args *SeckillSagaArgs) error
}
