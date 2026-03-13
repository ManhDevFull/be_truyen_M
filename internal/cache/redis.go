package cache

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func Connect(addr, password string, db int) (*redis.Client, error) {
	var opts *redis.Options
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		parsed, err := redis.ParseURL(addr)
		if err != nil {
			return nil, err
		}
		opts = parsed
		if password != "" {
			opts.Password = password
		}
		if db != 0 {
			opts.DB = db
		}
	} else {
		opts = &redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
