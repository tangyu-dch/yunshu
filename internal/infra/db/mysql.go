// Package db 提供基于 GORM 的数据库装配能力。
//
// 领域包只依赖小接口，GORM session、事务和模型映射留在基础设施层，避免 ORM 类型
// 扩散到业务规则里。
package db

import (
	"context"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 定义 MySQL 数据库连接配置，包含 DSN、连接池参数和连接生命周期限制。
type Config struct {
	DSN             string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

// OpenMySQL 打开 MySQL 连接并配置连接池。
func OpenMySQL(cfg Config) (*gorm.DB, error) {
	gormDB, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		sqlDB.SetMaxIdleConns(10) // 呼叫中心场景默认 10 个空闲连接
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		sqlDB.SetMaxOpenConns(100) // 默认最大 100 个连接
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	}
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return gormDB, nil
}

// WithTx 在统一事务边界内执行数据库操作。
// 涉及 DB 写入后发布消息的场景，应优先结合 outbox 使用。
func WithTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(fn)
}
