package logger

import "go.uber.org/zap"

func New(env string) (*zap.Logger, error) {
	if env == "prod" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
