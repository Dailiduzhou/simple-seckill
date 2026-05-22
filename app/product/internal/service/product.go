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
