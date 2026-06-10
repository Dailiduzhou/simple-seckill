package data

import (
	"context"

	productv1 "seckill/api/product/v1"
	userv1 "seckill/api/user/v1"
	"seckill/app/product/internal/biz"
	"seckill/app/product/internal/conf"

	"github.com/dtm-labs/client/dtmgrpc"
	_ "github.com/dtm-labs/driver-kratos"
)

type SAGAData struct {
	dtmAddr      string
	prodGrpcAddr string
	userGrpcAddr string
}

func NewSAGAData(dtmCfg *conf.Dtm) *SAGAData {
	return &SAGAData{
		dtmAddr:      dtmCfg.GetAddr(),
		prodGrpcAddr: dtmCfg.GetProdGrpcAddr(),
		userGrpcAddr: dtmCfg.GetUserGrpcAddr(),
	}
}

func (s *SAGAData) ExecuteSeckillSaga(ctx context.Context, args *biz.SeckillSagaArgs) error {
	saga := dtmgrpc.NewSagaGrpc(s.dtmAddr, args.GID).
		Add(
			s.userGrpcAddr+"/api.user.v1.User/DeductBalance",
			s.userGrpcAddr+"/api.user.v1.User/RestoreBalance",
			&userv1.DeductBalanceRequest{Id: args.UserID, Amount: int64(args.Amount)},
		).
		Add(
			s.prodGrpcAddr+"/api.product.v1.Product/DeductStockSaga",
			s.prodGrpcAddr+"/api.product.v1.Product/RestoreStock",
			&productv1.DeductStockSagaReq{ProductID: args.ProductID, Amount: args.Amount},
		)
	return saga.Submit()
}
