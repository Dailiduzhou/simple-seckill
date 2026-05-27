package data

import (
	"context"
	"fmt"
	"time"

	"seckill/app/product/internal/biz"
	"seckill/app/product/internal/conf"
	"seckill/app/product/internal/data/db"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"golang.org/x/sync/singleflight"
)

var ProviderSet = wire.NewSet(NewData, NewProductRepo, NewPgxPool, wire.Bind(new(biz.ProductRepo), new(*ProductRepo)))

type Data struct {
	pool        *pgxpool.Pool
	riverclient *river.Client[pgx.Tx]
	rdb         *redis.Client
	rs          *redsync.Redsync
	q           *db.Queries
	sg          *singleflight.Group
}

func NewPgxPool(c *conf.Data) (*pgxpool.Pool, error) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, c.Database.Source)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

func NewData(c *conf.Data, pool *pgxpool.Pool, riverClient *river.Client[pgx.Tx]) (*Data, func(), error) {
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Addr,
		Password: "",
		DB:       0,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, nil, fmt.Errorf("ping redis: %w", err)
	}

	redisPool := goredis.NewPool(rdb)
	rs := redsync.New(redisPool)

	cleanup := func() {
		riverClient.Stop(ctx)
		rdb.Close()
		pool.Close()

		log.Info("closing the data resources")
	}
	return &Data{
		pool:        pool,
		riverclient: riverClient,
		rdb:         rdb,
		rs:          rs,
		q:           db.New(pool),
		sg:          &singleflight.Group{},
	}, cleanup, nil
}
