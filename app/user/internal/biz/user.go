package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type User struct {
	ID      int64
	Balance int32
}

type UserRepo interface {
	Create(ctx context.Context) (*User, error)
	FindByID(ctx context.Context, ID int64) (*User, error)
	DeducBalance(ctx context.Context, ID int64, amount int32) error
	RestoreBalance(ctx context.Context, ID int64, amount int32) error
}

type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *UserUsecase) Register(ctx context.Context) (*User, error) {
	uc.log.WithContext(ctx).Info("Register: creating user")
	user, err := uc.repo.Create(ctx)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("Register: %v", err)
		return nil, err
	}
	uc.log.WithContext(ctx).Infof("Register: created user id=%d", user.ID)
	return user, nil
}

func (uc *UserUsecase) GetUser(ctx context.Context, ID int64) (*User, error) {
	user, err := uc.repo.FindByID(ctx, ID)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("GetUser: id=%d %v", ID, err)
		return nil, err
	}
	return user, nil
}

func (uc *UserUsecase) DeducBalance(ctx context.Context, ID int64, amount int32) error {
	uc.log.WithContext(ctx).Infof("DeducBalance: user_id=%d amount=%d", ID, amount)
	if err := uc.repo.DeducBalance(ctx, ID, amount); err != nil {
		uc.log.WithContext(ctx).Errorf("DeducBalance: user_id=%d amount=%d %v", ID, amount, err)
		return err
	}
	return nil
}

func (uc *UserUsecase) RestoreBalance(ctx context.Context, ID int64, amount int32) error {
	uc.log.WithContext(ctx).Infof("RestoreBalance: user_id=%d amount=%d", ID, amount)
	if err := uc.repo.RestoreBalance(ctx, ID, amount); err != nil {
		uc.log.WithContext(ctx).Errorf("RestoreBalance: user_id=%d amount=%d %v", ID, amount, err)
		return err
	}
	return nil
}
