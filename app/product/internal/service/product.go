package service

import (
	"context"

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
	err := s.uc.Seckill(ctx, req.UserID)
	if err != nil {
		s.log.WithContext(ctx).Errorf("Seckill: user_id=%d %v", req.UserID, err)
		return &pb.SeckillResp{Res: pb.Result_FAILURE}, err
	}
	return &pb.SeckillResp{Res: pb.Result_SUCCESS}, nil
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
