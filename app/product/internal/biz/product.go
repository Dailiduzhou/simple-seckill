package biz

import (
	"context"
	"fmt"
	"time"

	productv1 "seckill/api/product/v1"
	userv1 "seckill/api/user/v1"
	"seckill/app/product/internal/conf"

	"github.com/dtm-labs/client/dtmgrpc"
	_ "github.com/dtm-labs/driver-kratos"
	"github.com/go-kratos/kratos/v2/log"
)

type Product struct {
	ID    int64
	Price int32
	Stock int32
}

type ProductRepo interface {
	FindByID(ctx context.Context, ID int64) (*Product, error)
	DeductStockSaga(ctx context.Context, productID int64, amount int32) error
	RestoreStock(ctx context.Context, productID int64, amount int32) error
}

type ProductUsecase struct {
	repo   ProductRepo
	dtmCfg *conf.Dtm
	log    *log.Helper
}

func NewProductUsecase(repo ProductRepo, dtmCfg *conf.Dtm, logger log.Logger) *ProductUsecase {
	if dtmCfg == nil || dtmCfg.Addr == "" {
		panic("dmt config is required")
	}
	return &ProductUsecase{
		repo:   repo,
		dtmCfg: dtmCfg,
		log:    log.NewHelper(logger),
	}
}

func (uc *ProductUsecase) DeductStockSaga(ctx context.Context, productID int64, amount int32) error {
	return uc.repo.DeductStockSaga(ctx, productID, amount)
}

func (uc *ProductUsecase) RestoreStock(ctx context.Context, productID int64, amount int32) error {
	return uc.repo.RestoreStock(ctx, productID, amount)
}

func (uc *ProductUsecase) Seckill(ctx context.Context, userID int64) error {
	uc.log.WithContext(ctx).Infof("Seckill: user_id=%d via DTM SAGA", userID)

	product, err := uc.repo.FindByID(ctx, 1)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("Seckill: find product: %v", err)
		return err
	}

	if product.Stock == 0 {
		uc.log.WithContext(ctx).Infof("Seckill: product sold out")
		return productv1.ErrorSoldOut("product sold out")
	}

	gid := fmt.Sprintf("seckill_%d_%d", userID, time.Now().UnixNano())

	saga := dtmgrpc.NewSagaGrpc(uc.dtmCfg.Addr, gid).
		Add(
			uc.dtmCfg.ProdGrpcAddr+"/api.product.v1.Product/DeductStockSaga",
			uc.dtmCfg.ProdGrpcAddr+"/api.product.v1.Product/RestoreStock",
			&productv1.DeductStockSagaReq{ProductID: 1, Amount: 1},
		).
		Add(
			uc.dtmCfg.UserGrpcAddr+"/api.user.v1.User/DeductBalance",
			uc.dtmCfg.UserGrpcAddr+"/api.user.v1.User/RestoreBalance",
			&userv1.DeductBalanceRequest{Id: userID, Amount: int64(product.Price)},
		)

	if err := saga.Submit(); err != nil {
		uc.log.WithContext(ctx).Errorf("Seckill: SAGA submit failed: %v", err)
		return err
	}

	return nil
}
