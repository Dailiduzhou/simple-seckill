package service

import (
	"context"
	"time"

	pb "seckill/api/product/v1"
	"seckill/app/product/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type ProductService struct {
	pb.UnimplementedProductServer
	uc  *biz.ProductUsecase
	log *log.Helper
}

func NewProductService(uc *biz.ProductUsecase, logger log.Logger) *ProductService {
	return &ProductService{uc: uc, log: log.NewHelper(logger)}
}

func (s *ProductService) Seckill(ctx context.Context, req *pb.SeckillReq) (*pb.SeckillResp, error) {
	start := time.Now()
	result := "queued"
	errMsg := ""
	defer func() {
		s.log.WithContext(ctx).Infow(
			"event", "seckill_request",
			"service", "product",
			"method", "ProductService.Seckill",
			"user_id", req.UserID,
			"result", result,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", errMsg,
		)
	}()

	request, err := s.uc.EnqueueSeckill(ctx, req.UserID)
	if err != nil {
		result = "error"
		errMsg = err.Error()
		s.log.WithContext(ctx).Errorf("Seckill: user_id=%d %v", req.UserID, err)
		return &pb.SeckillResp{Res: pb.Result_FAILURE}, err
	}
	return &pb.SeckillResp{Res: pb.Result_QUEUED, RequestID: request.RequestID}, nil
}

func (s *ProductService) DeductStockSaga(ctx context.Context, req *pb.DeductStockSagaReq) (*pb.DeductStockSagaResp, error) {
	err := s.uc.DeductStockSaga(ctx, req.ProductID, req.Amount)
	if err != nil {
		s.log.WithContext(ctx).Errorf("DeductStockSaga: product_id=%d amount=%d %v", req.ProductID, req.Amount, err)
		return &pb.DeductStockSagaResp{Success: false}, err
	}
	return &pb.DeductStockSagaResp{Success: true}, nil
}

func (s *ProductService) RestoreStock(ctx context.Context, req *pb.RestoreStockReq) (*pb.RestoreStockResp, error) {
	err := s.uc.RestoreStock(ctx, req.ProductID, req.Amount)
	if err != nil {
		s.log.WithContext(ctx).Errorf("RestoreStock: product_id=%d amount=%d %v", req.ProductID, req.Amount, err)
		return &pb.RestoreStockResp{Success: false}, err
	}
	return &pb.RestoreStockResp{Success: true}, nil
}

func (s *ProductService) GetProduct(ctx context.Context, req *pb.GetProductReq) (*pb.GetProductResp, error) {
	product, err := s.uc.GetProduct(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	return &pb.GetProductResp{
		ProductID: product.ID,
		Price:     product.Price,
		Stock:     product.Stock,
	}, nil
}

func (s *ProductService) GetSeckillStatus(ctx context.Context, req *pb.GetSeckillStatusReq) (*pb.GetSeckillStatusResp, error) {
	request, err := s.uc.GetSeckillStatus(ctx, req.RequestID)
	if err != nil {
		return nil, err
	}
	return &pb.GetSeckillStatusResp{
		Status:    biz.MapSeckillStatus(request.Status),
		Reason:    request.Reason,
		UserID:    request.UserID,
		ProductID: request.ProductID,
	}, nil
}
