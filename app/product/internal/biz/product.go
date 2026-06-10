package biz

import (
	"context"
	"fmt"
	"time"

	productv1 "seckill/api/product/v1"
	"seckill/app/product/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	StatusWaiting    = "WAITING"
	StatusProcessing = "PROCESSING"
	StatusSuccess    = "SUCCESS"
	StatusSoldOut    = "SOLD_OUT"
	StatusDegraded   = "DEGRADED"
	StatusFailed     = "FAILED"
)

type Product struct {
	ID    int64
	Price int32
	Stock int32
}

type SeckillRequest struct {
	RequestID string
	UserID    int64
	ProductID int64
	Status    string
	Reason    string
}

// ProductRepo is the data access interface for product CRUD and stock operations.
type ProductRepo interface {
	FindByID(ctx context.Context, ID int64) (*Product, error)
	DeductStock(ctx context.Context, productID int64, amount int32) error
	RestoreStock(ctx context.Context, productID int64, amount int32) error
}

// SeckillRequestRepo manages seckill request status in the cache.
type SeckillRequestRepo interface {
	CreateSeckillRequest(ctx context.Context, request *SeckillRequest, ttl time.Duration) error
	GetSeckillRequest(ctx context.Context, requestID string) (*SeckillRequest, error)
	UpdateSeckillRequest(ctx context.Context, requestID, status, reason string) error
}

// SeckillJobRepo manages job queue insertion.
type SeckillJobRepo interface {
	InsertSeckillJob(ctx context.Context, args *SeckillArgs, queueName string) error
	InsertMessagingJob(ctx context.Context, args *MessagingArgs) error
}

// SeckillHandler is the interface that SeckillWorker depends on to process a seckill job.
type SeckillHandler interface {
	HandleSeckill(ctx context.Context, args *SeckillArgs) error
}

type MessagingArgs struct {
	ProductID int64 `json:"product_id"`
	Amount    int32 `json:"amount"`
}

func (MessagingArgs) Kind() string { return "order.messaging" }

type SeckillArgs struct {
	RequestID  string `json:"request_id" river:"unique"`
	UserID     int64  `json:"user_id"`
	ProductID  int64  `json:"product_id"`
	Amount     int32  `json:"amount"`
	EnqueuedAt int64  `json:"enqueued_at"`
}

func (SeckillArgs) Kind() string { return "seckill.request" }

type ProductUsecase struct {
	productRepo    ProductRepo
	seckillReqRepo SeckillRequestRepo
	jobRepo        SeckillJobRepo
	productID      int64
	amount         int32
	maxWait        time.Duration
	queueName      string
	queueWorkers   int
	log            *log.Helper
}

func NewProductUsecase(
	productRepo ProductRepo,
	seckillReqRepo SeckillRequestRepo,
	jobRepo SeckillJobRepo,
	skcCfg *conf.Seckill,
	logger log.Logger,
) *ProductUsecase {
	productID := skcCfg.GetProductId()
	if productID == 0 {
		productID = 1
	}
	amount := skcCfg.GetAmount()
	if amount == 0 {
		amount = 1
	}
	maxWait := skcCfg.GetMaxWait().AsDuration()
	queueName := skcCfg.GetQueueName()
	if queueName == "" {
		queueName = "seckill"
	}
	queueWorkers := int(skcCfg.GetQueueWorkers())
	if queueWorkers == 0 {
		queueWorkers = 10
	}
	return &ProductUsecase{
		productRepo:    productRepo,
		seckillReqRepo: seckillReqRepo,
		jobRepo:        jobRepo,
		productID:      productID,
		amount:         amount,
		maxWait:        maxWait,
		queueName:      queueName,
		queueWorkers:   queueWorkers,
		log:            log.NewHelper(logger),
	}
}

func (uc *ProductUsecase) MaxWait() time.Duration {
	return uc.maxWait
}

// HandleSeckill processes a dequeued seckill job.
func (uc *ProductUsecase) HandleSeckill(ctx context.Context, args *SeckillArgs) error {
	waited := time.Since(time.UnixMilli(args.EnqueuedAt))
	if waited > uc.maxWait {
		return uc.seckillReqRepo.UpdateSeckillRequest(ctx, args.RequestID, StatusDegraded, fmt.Sprintf("queued for %s", waited))
	}

	if err := uc.seckillReqRepo.UpdateSeckillRequest(ctx, args.RequestID, StatusProcessing, ""); err != nil {
		return err
	}

	err := uc.executeSeckill(ctx, args.UserID, args.ProductID, args.Amount)
	if err != nil {
		status := StatusFailed
		if productv1.IsSoldOut(err) {
			status = StatusSoldOut
		}
		if updateErr := uc.seckillReqRepo.UpdateSeckillRequest(ctx, args.RequestID, status, err.Error()); updateErr != nil {
			uc.log.WithContext(ctx).Errorf("update seckill request %s failed: %v", args.RequestID, updateErr)
		}
		return nil
	}

	return uc.seckillReqRepo.UpdateSeckillRequest(ctx, args.RequestID, StatusSuccess, "")
}

// DeductStockSaga is the SAGA forward action for stock deduction.
func (uc *ProductUsecase) DeductStockSaga(ctx context.Context, productID int64, amount int32) error {
	return uc.productRepo.DeductStock(ctx, productID, amount)
}

// RestoreStock is the SAGA compensation action for stock restoration.
func (uc *ProductUsecase) RestoreStock(ctx context.Context, productID int64, amount int32) error {
	return uc.productRepo.RestoreStock(ctx, productID, amount)
}

// EnqueueSeckill atomically creates a seckill request and inserts the job.
func (uc *ProductUsecase) EnqueueSeckill(ctx context.Context, userID int64) (*SeckillRequest, error) {
	requestID := fmt.Sprintf("seckill_%d_%d", userID, time.Now().UnixNano())
	request := &SeckillRequest{
		RequestID: requestID,
		UserID:    userID,
		ProductID: uc.productID,
		Status:    StatusWaiting,
	}

	ttl := uc.maxWait + time.Hour
	if err := uc.seckillReqRepo.CreateSeckillRequest(ctx, request, ttl); err != nil {
		return nil, err
	}

	args := &SeckillArgs{
		RequestID:  requestID,
		UserID:     userID,
		ProductID:  uc.productID,
		Amount:     uc.amount,
		EnqueuedAt: time.Now().UnixMilli(),
	}

	if err := uc.jobRepo.InsertSeckillJob(ctx, args, uc.queueName); err != nil {
		_ = uc.seckillReqRepo.UpdateSeckillRequest(ctx, requestID, StatusFailed, err.Error())
		return request, err
	}
	return request, nil
}

func (uc *ProductUsecase) GetSeckillStatus(ctx context.Context, requestID string) (*SeckillRequest, error) {
	return uc.seckillReqRepo.GetSeckillRequest(ctx, requestID)
}

func (uc *ProductUsecase) GetProduct(ctx context.Context, productID int64) (*Product, error) {
	return uc.productRepo.FindByID(ctx, productID)
}

func (uc *ProductUsecase) executeSeckill(ctx context.Context, userID, productID int64, amount int32) error {
	uc.log.WithContext(ctx).Infof("Seckill: user_id=%d", userID)

	if err := uc.productRepo.DeductStock(ctx, productID, amount); err != nil {
		uc.log.WithContext(ctx).Errorf("Seckill: deduct stock: %v", err)
		return err
	}

	if err := uc.jobRepo.InsertMessagingJob(ctx, &MessagingArgs{ProductID: productID, Amount: amount}); err != nil {
		uc.log.WithContext(ctx).Errorf("Seckill: insert messaging job: %v", err)
	}

	return nil
}

func MapSeckillStatus(status string) productv1.SeckillStatus {
	switch status {
	case StatusWaiting:
		return productv1.SeckillStatus_SECKILL_STATUS_WAITING
	case StatusProcessing:
		return productv1.SeckillStatus_SECKILL_STATUS_PROCESSING
	case StatusSuccess:
		return productv1.SeckillStatus_SECKILL_STATUS_SUCCESS
	case StatusSoldOut:
		return productv1.SeckillStatus_SECKILL_STATUS_SOLD_OUT
	case StatusDegraded:
		return productv1.SeckillStatus_SECKILL_STATUS_DEGRADED
	default:
		return productv1.SeckillStatus_SECKILL_STATUS_FAILED
	}
}
