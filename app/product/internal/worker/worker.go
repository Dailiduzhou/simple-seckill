package worker

import (
	"context"

	"seckill/app/product/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/riverqueue/river"
)

var ProviderSet = wire.NewSet(NewSeckillWorker, NewMessagingWorker)

type SeckillWorker struct {
	river.WorkerDefaults[biz.SeckillArgs]
	handler biz.SeckillHandler
	log     *log.Helper
}

func NewSeckillWorker(handler biz.SeckillHandler, logger log.Logger) *SeckillWorker {
	return &SeckillWorker{handler: handler, log: log.NewHelper(logger)}
}

func (w *SeckillWorker) Work(ctx context.Context, job *river.Job[biz.SeckillArgs]) error {
	args := job.Args
	return w.handler.HandleSeckill(ctx, &args)
}

type MessagingWorker struct {
	river.WorkerDefaults[biz.MessagingArgs]
	log *log.Helper
}

func NewMessagingWorker(logger log.Logger) *MessagingWorker {
	return &MessagingWorker{log: log.NewHelper(logger)}
}

func (w *MessagingWorker) Work(ctx context.Context, job *river.Job[biz.MessagingArgs]) error {
	args := job.Args

	w.log.WithContext(ctx).Infof("No. %d product stock has deducted by %d", args.ProductID, args.Amount)
	return nil
}
