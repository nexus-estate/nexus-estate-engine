package logging

import "go.uber.org/zap"

func New(service, env string) (*zap.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return logger.With(zap.String("service", service), zap.String("env", env)), nil
}
