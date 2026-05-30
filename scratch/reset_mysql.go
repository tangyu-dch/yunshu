package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"yunshu/internal/infra/installer"
)

func main() {
	// 连接 MySQL，不指定任何数据库，以便物理清空所有旧库
	dsn := "root:db123456@tcp(127.0.0.1:3306)/?charset=utf8mb4&parseTime=True&loc=Local"
	fmt.Printf("[MySQL 重置工具] 正在连接数据库进行物理重置: %s\n", dsn)

	dbConn, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("[MySQL 重置工具] 连接失败: %v\n", err)
	}
	defer dbConn.Close()

	// 1. 物理清退所有旧库以保证彻底纯净
	legacyDBs := []string{"dolphin", "callcenter", "yunshu"}
	for _, dbName := range legacyDBs {
		_, err := dbConn.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		if err != nil {
			log.Fatalf("[MySQL 重置工具] 物理清退历史库 %s 失败: %v\n", dbName, err)
		}
	}
	fmt.Println("[MySQL 重置工具] 历史数据库 (dolphin, callcenter, yunshu) 物理清退成功！")

	// 2. 利用 Installer 执行一键自动建表与全量播种（单进程安全隔离，防并发 DDL 冲突）
	inst := installer.NewInstaller(slog.Default())
	params := installer.SetupParams{
		MySQLHost:         "127.0.0.1",
		MySQLPort:         3306,
		MySQLUser:         "root",
		MySQLPassword:     "db123456",
		MySQLDatabase:     "yunshu",
		DefaultMerchantID: 1001,
	}

	fmt.Println("[MySQL 重置工具] 正在调起核心 Installer 进行单进程自动建表及统一种子数据播种...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := inst.InitializeDatabase(ctx, params); err != nil {
		log.Fatalf("[MySQL 重置工具] 自动建表与种子数据播种失败: %v\n", err)
	}

	fmt.Println("[MySQL 重置工具] =================================================================")
	fmt.Println("[MySQL 重置工具] ★★★ 数据库清空、全量 50 张表自动生成、Unified Seeder 灌入 100% 成功！ ★★★")
	fmt.Println("[MySQL 重置工具] =================================================================")
}
