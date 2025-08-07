package mysql

import (
	"backend/config"
	"backend/internal/model/entity"
	"fmt"
	"net/url"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

func Init(cfg *config.MySQLConfig) (err error) {
	// 构建数据库连接DSN(Data Source Name)字符串
	// 注意：对密码进行URL编码，防止特殊字符导致解析失败
	escapedPassword := url.QueryEscape(cfg.Password)
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		cfg.User,        // 用户名
		escapedPassword, // 密码（已转义特殊字符）
		cfg.Host,        // 主机（IP 或域名）
		cfg.Port,        // 端口（整数）
		cfg.DB,          // 数据库名
	)

	// 使用GORM建立数据库连接
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		// 添加预编译语句设置提升性能（可选）
		PrepareStmt: true,
	})
	if err != nil {
		return // 连接失败直接返回错误
	}

	// 获取底层SQL数据库对象(*sql.DB)以配置连接池
	// GORM底层使用的是database/sql，需要通过DB()方法获取
	sqlDB, err := db.DB()
	if err != nil {
		return // 获取失败返回错误
	}

	sqlDB.SetMaxOpenConns(config.Conf.MaxOpenConns) // 设置最大打开连接数
	sqlDB.SetMaxIdleConns(config.Conf.MaxIdleConns) // 设置最大空闲连接数
	sqlDB.SetConnMaxLifetime(30 * time.Minute)      // 添加连接最大生命周期

	// 新增：执行Ping测试确保连接可用
	if err = sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	entity.AutoMigrateUser(db)
	entity.AutoMigrateSpace(db)
	entity.AutoMigrateSpaceUser(db)
	entity.AutoMigratePicture(db)
	entity.AutoMigrateITask(db)
	return nil
}

// Close 关闭MySQL连接
// 添加超时控制和错误处理

func Close() {
	sqlDB, err := db.DB()
	if err != nil {
		return // 获取失败则直接返回，不作处理
	}

	// 关闭数据库连接(忽略可能的关闭错误)
	_ = sqlDB.Close()
}

func LoadDB() *gorm.DB {
	return db.Session(&gorm.Session{})
}
