package data

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math/rand"
	"time"

	productv1 "seckill/api/product/v1"
	"seckill/app/product/internal/biz"
	"seckill/app/product/internal/data/db"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

// ProductRepoImpl implements biz.ProductRepo.
type ProductRepoImpl struct {
	data *Data
	log  *log.Helper
}

func NewProductRepo(data *Data, logger log.Logger) *ProductRepoImpl {
	return &ProductRepoImpl{data: data, log: log.NewHelper(logger)}
}

func (r *ProductRepoImpl) FindByID(ctx context.Context, ID int64) (*biz.Product, error) {
	cacheKey := fmt.Sprintf("product:%d", ID)

	product, err := r.getCache(ctx, cacheKey)
	if err == nil {
		return product, nil
	}
	if !stderrors.Is(err, redis.Nil) {
		r.log.WithContext(ctx).Errorf("get product cache: %v", err)
	}

	sfKey := fmt.Sprintf("sf:product:%d", ID)
	val, err, _ := r.data.sg.Do(sfKey, func() (interface{}, error) {
		lockKey := fmt.Sprintf("lock:product:%d", ID)
		mutex := r.data.rs.NewMutex(lockKey, redsync.WithExpiry(5*time.Second))

		if err := mutex.LockContext(ctx); err != nil {
			time.Sleep(100 * time.Millisecond)
			return r.getCache(ctx, cacheKey)
		}
		defer mutex.Unlock()

		productDoublecheck, err := r.getCache(ctx, cacheKey)
		if err == nil {
			return productDoublecheck, nil
		}

		r.log.WithContext(ctx).Infof("product %d fetching from DB", ID)
		dbProduct, err := r.data.q.GetProduct(ctx, ID)
		if err != nil {
			if stderrors.Is(err, pgx.ErrNoRows) {
				return nil, errors.InternalServer("DB_ERROR", "no product")
			}
			return nil, errors.InternalServer("DB_ERROR", "failed to fetch product")
		}
		finalProduct := &biz.Product{
			ID:    dbProduct.ID,
			Price: dbProduct.Price,
			Stock: dbProduct.Stock,
		}
		r.setCache(ctx, cacheKey, finalProduct)
		return finalProduct, nil
	})

	if err != nil {
		return nil, err
	}

	return val.(*biz.Product), nil
}

func (r *ProductRepoImpl) DeductStock(ctx context.Context, productID int64, amount int32) error {
	rows, err := r.data.q.DeductStock(ctx, db.DeductStockParams{
		ID:    productID,
		Stock: amount,
	})
	if err != nil {
		return errors.InternalServer("DB_ERROR", "failed to deduct stock")
	}
	if rows == 0 {
		return productv1.ErrorSoldOut("product %d sold out or not found", productID)
	}

	cacheKey := fmt.Sprintf("product:%d", productID)
	if err := r.data.rdb.Del(ctx, cacheKey).Err(); err != nil {
		r.log.WithContext(ctx).Errorf("delete product cache after deduct: %v", err)
	}
	return nil
}

func (r *ProductRepoImpl) RestoreStock(ctx context.Context, productID int64, amount int32) error {
	rows, err := r.data.q.RestoreStock(ctx, db.RestoreStockParams{
		ID:    productID,
		Stock: amount,
	})
	if err != nil {
		return errors.InternalServer("DB_ERROR", "failed to restore stock")
	}
	if rows == 0 {
		r.log.WithContext(ctx).Warnf("RestoreStock: 0 rows affected for product %d", productID)
	}

	cacheKey := fmt.Sprintf("product:%d", productID)
	if err := r.data.rdb.Del(ctx, cacheKey).Err(); err != nil {
		r.log.WithContext(ctx).Errorf("delete product cache after restore: %v", err)
	}
	return nil
}

func (r *ProductRepoImpl) getCache(ctx context.Context, key string) (*biz.Product, error) {
	val, err := r.data.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var product biz.Product
	if err := json.Unmarshal(val, &product); err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepoImpl) setCache(ctx context.Context, key string, product *biz.Product) {
	data, err := json.Marshal(product)
	if err != nil {
		r.log.WithContext(ctx).Errorf("marshal product cache: %v", err)
		return
	}
	jitter := time.Duration(rand.Intn(10)) * time.Minute
	exp := jitter + 10*time.Minute
	r.data.rdb.Set(ctx, key, data, exp)
}

// SeckillRequestRepoImpl implements biz.SeckillRequestRepo using Redis.
type SeckillRequestRepoImpl struct {
	data *Data
	log  *log.Helper
}

func NewSeckillRequestRepo(data *Data, logger log.Logger) *SeckillRequestRepoImpl {
	return &SeckillRequestRepoImpl{data: data, log: log.NewHelper(logger)}
}

func (r *SeckillRequestRepoImpl) CreateSeckillRequest(ctx context.Context, request *biz.SeckillRequest, ttl time.Duration) error {
	key := r.seckillRequestKey(request.RequestID)
	payload, err := json.Marshal(request)
	if err != nil {
		return errors.InternalServer("CACHE_ERROR", "failed to marshal seckill request")
	}
	ok, err := r.data.rdb.SetNX(ctx, key, payload, ttl).Result()
	if err != nil {
		return errors.InternalServer("CACHE_ERROR", "failed to create seckill request")
	}
	if !ok {
		return productv1.ErrorQueueDuplicate("duplicate seckill request")
	}
	return nil
}

func (r *SeckillRequestRepoImpl) GetSeckillRequest(ctx context.Context, requestID string) (*biz.SeckillRequest, error) {
	val, err := r.data.rdb.Get(ctx, r.seckillRequestKey(requestID)).Bytes()
	if err != nil {
		if stderrors.Is(err, redis.Nil) {
			return nil, productv1.ErrorRequestNotFound("request %s not found", requestID)
		}
		return nil, errors.InternalServer("CACHE_ERROR", "failed to fetch seckill request")
	}

	var request biz.SeckillRequest
	if err := json.Unmarshal(val, &request); err != nil {
		return nil, errors.InternalServer("CACHE_ERROR", "failed to decode seckill request")
	}
	return &request, nil
}

// updateSeckillRequestScript atomically updates status+reason while preserving TTL.
const updateSeckillRequestScript = `
local val = redis.call('GET', KEYS[1])
if not val then return 0 end
local req = cjson.decode(val)
req.Status = ARGV[1]
req.Reason = ARGV[2]
local ttl = redis.call('TTL', KEYS[1])
if ttl <= 0 then ttl = 3600 end
redis.call('SET', KEYS[1], cjson.encode(req), 'EX', ttl)
return 1
`

func (r *SeckillRequestRepoImpl) UpdateSeckillRequest(ctx context.Context, requestID, status, reason string) error {
	key := r.seckillRequestKey(requestID)
	result, err := r.data.rdb.Eval(ctx, updateSeckillRequestScript, []string{key}, status, reason).Result()
	if err != nil {
		return errors.InternalServer("CACHE_ERROR", "failed to update seckill request")
	}
	if n, _ := result.(int64); n == 0 {
		return productv1.ErrorRequestNotFound("request %s not found", requestID)
	}
	return nil
}

func (r *SeckillRequestRepoImpl) seckillRequestKey(requestID string) string {
	return fmt.Sprintf("seckill:request:%s", requestID)
}

// SeckillJobRepoImpl implements biz.SeckillJobRepo using River.
type SeckillJobRepoImpl struct {
	riverclient *river.Client[pgx.Tx]
}

func NewSeckillJobRepo(riverClient *river.Client[pgx.Tx]) *SeckillJobRepoImpl {
	return &SeckillJobRepoImpl{riverclient: riverClient}
}

func (r *SeckillJobRepoImpl) InsertSeckillJob(ctx context.Context, args *biz.SeckillArgs, queueName string) error {
	_, err := r.riverclient.Insert(ctx, args, &river.InsertOpts{
		Queue: queueName,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	})
	return err
}

func (r *SeckillJobRepoImpl) InsertMessagingJob(ctx context.Context, args *biz.MessagingArgs) error {
	_, err := r.riverclient.Insert(ctx, args, nil)
	if err != nil {
		return fmt.Errorf("insert messaging job: %w", err)
	}
	return nil
}
