package service

import "context"

type NovaService struct{}

func (s *NovaService) Handle(ctx context.Context, data []byte) ([]byte, error) {
	// 业务逻辑
	return data, nil
}
