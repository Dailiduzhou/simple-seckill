package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type Product struct {
	ID    int64
	Price int32
	Stock int32
}

type ProductRepo interface {
	FindByID(ctx context.Context, ID int64) (*Product, error)
	DeductStock(ctx context.Context, userID int64, ID int64, amount int32) error
}

type ProductUsecase struct {
	repo ProductRepo
	log  *log.Helper
}

func NewProductUsecase(repo ProductRepo, logger log.Logger) *ProductUsecase {
	return &ProductUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *ProductUsecase) Seckill(ctx context.Context, userID int64) error {
	uc.log.WithContext(ctx).Infof("Seckill: user_id=%d", userID)
	if err := uc.repo.DeductStock(ctx, userID, 1, 1); err != nil {
		uc.log.WithContext(ctx).Errorf("Seckill: user_id=%d %v", userID, err)
		return err
	}
	return nil
}
