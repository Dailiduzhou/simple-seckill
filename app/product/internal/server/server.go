package server

import (
	"context"
	"fmt"
	"time"

	"seckill/app/product/internal/conf"
	"seckill/app/product/internal/worker"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

var ProviderSet = wire.NewSet(NewGRPCServer, NewHTTPServer, NewEtcdClient, NewDiscovery, NewRegistrar, NewRiverClient, NewRiverServer, NewRiverWorkers)

// NewRiverWorkers creates an empty Workers container. Workers are registered
// after the full dependency graph is wired via RegisterRiverWorkers.
func NewRiverWorkers() *river.Workers {
	return river.NewWorkers()
}

// RegisterRiverWorkers adds workers to the container after all dependencies are wired.
func RegisterRiverWorkers(workers *river.Workers, sw *worker.SeckillWorker, mw *worker.MessagingWorker) {
	river.AddWorker(workers, sw)
	river.AddWorker(workers, mw)
}

func NewRiverClient(pool *pgxpool.Pool, workers *river.Workers, seckillCfg *conf.Seckill) (*river.Client[pgx.Tx], error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	queueName := seckillCfg.GetQueueName()
	if queueName == "" {
		queueName = "seckill"
	}
	queueWorkers := int(seckillCfg.GetQueueWorkers())
	if queueWorkers <= 0 {
		queueWorkers = 10
	}

	driver := riverpgxv5.New(pool)
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate river schema: %w", err)
	}

	riverClient, err := river.NewClient(driver, &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
			queueName:           {MaxWorkers: queueWorkers},
		},
		Workers: workers,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return riverClient, nil
}

type RiverServer struct {
	client *river.Client[pgx.Tx]
}

func NewRiverServer(riverClient *river.Client[pgx.Tx]) *RiverServer {
	return &RiverServer{client: riverClient}
}

func (s *RiverServer) Start(ctx context.Context) error {
	return s.client.Start(ctx)
}

func (s *RiverServer) Stop(ctx context.Context) error {
	return s.client.Stop(ctx)
}
