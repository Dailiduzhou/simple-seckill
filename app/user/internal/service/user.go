package service

import (
	"context"

	pb "seckill/api/user/v1"
	"seckill/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type UserService struct {
	pb.UnimplementedUserServer
	uc  *biz.UserUsecase
	log *log.Helper
}

func NewUserService(uc *biz.UserUsecase, logger log.Logger) *UserService {
	return &UserService{uc: uc, log: log.NewHelper(logger)}
}

func (s *UserService) Register(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserReply, error) {
	user, err := s.uc.Register(ctx)
	if err != nil {
		s.log.WithContext(ctx).Errorf("Register: %v", err)
		return nil, err
	}
	return &pb.CreateUserReply{Id: user.ID}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserReply, error) {
	user, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		s.log.WithContext(ctx).Errorf("GetUser: id=%d %v", req.Id, err)
		return nil, err
	}
	return &pb.GetUserReply{Id: user.ID}, nil
}

func (s *UserService) DeductBalance(ctx context.Context, req *pb.DeductBalanceRequest) (*pb.DeductBalanceReply, error) {
	err := s.uc.DeducBalance(ctx, req.Id, int32(req.Amount))
	if err != nil {
		s.log.WithContext(ctx).Errorf("DeductBalance: id=%d amount=%d %v", req.Id, req.Amount, err)
		return &pb.DeductBalanceReply{Success: false}, err
	}
	return &pb.DeductBalanceReply{Success: true}, nil
}
