//给全项目开好 MySQL 大门，后续所有查库改库都用 DB 对象
//连接池初始化 + 自动建数据表
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB     //sql.DB 是 Go 标准库封装的数据库连接池结构体

func InitMySQL() error {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		getEnv("DB_USER", "root"),      //getEnv:作用：读取系统环境变量，常用于配置分离
		getEnv("DB_PASSWORD", "root"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_NAME", "seckill"),
	)

	var err error
	DB, err = sql.Open("mysql", dsn)     //sql.Open不建立真实连接，一定要调用 DB.Ping() 验证连通性
	//sql.Open 在堆上创建一个连接池实例，把实例地址赋值给全局 DB，此时 DB 不再是 nil。
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}

	DB.SetMaxOpenConns(50)      //配置连接池参数   最大并发连接50
	DB.SetMaxIdleConns(10)      //空闲连接保留10个
	DB.SetConnMaxLifetime(time.Hour)       //连接1小时过期销毁

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	log.Println("[DB] MySQL 连接成功")
	return nil
}

 
func RunMigrations() error {        //自动建表函数   重点秒杀表结构  不用手动去数据库建表，项目启动自动生成表结构。
	schema := []string{
		`CREATE TABLE IF NOT EXISTS buyers (
		buyer_id INT AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		password VARCHAR(255) NOT NULL,
		phone VARCHAR(20) NOT NULL UNIQUE,
		address TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS sellers (
		seller_id INT AUTO_INCREMENT PRIMARY KEY,
		shop_name VARCHAR(100) NOT NULL,
		password VARCHAR(255) NOT NULL,
		phone VARCHAR(20) NOT NULL UNIQUE,
		shop_addr TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS seckill_products (
		product_id INT AUTO_INCREMENT PRIMARY KEY,
		seller_id INT NOT NULL,
		name VARCHAR(200) NOT NULL,
		description TEXT,
		original_price DECIMAL(10,2) NOT NULL,
		seckill_price DECIMAL(10,2) NOT NULL,
		stock INT NOT NULL,
		seckill_start TIMESTAMP NOT NULL,
		seckill_end TIMESTAMP NOT NULL,
		status VARCHAR(20) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		FOREIGN KEY (seller_id) REFERENCES sellers(seller_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS seckill_orders (
		order_id VARCHAR(64) PRIMARY KEY,
		buyer_id INT NOT NULL,
		seller_id INT NOT NULL,
		product_id INT NOT NULL,
		product_name VARCHAR(200) NOT NULL,
		seckill_price DECIMAL(10,2) NOT NULL,
		order_status VARCHAR(20) DEFAULT 'paid',
		buyer_name VARCHAR(50) NOT NULL,
		seller_name VARCHAR(100) NOT NULL,
		order_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		FOREIGN KEY (buyer_id) REFERENCES buyers(buyer_id),
		FOREIGN KEY (seller_id) REFERENCES sellers(seller_id),
		FOREIGN KEY (product_id) REFERENCES seckill_products(product_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// 聊天会话表：记录两个人之间的聊天关系
		// 同一对用户只会有一个会话，用 UNIQUE 索引保证
		`CREATE TABLE IF NOT EXISTS conversations (
		conversation_id INT AUTO_INCREMENT PRIMARY KEY,
		user1_id INT NOT NULL,
		user1_role VARCHAR(10) NOT NULL,
		user2_id INT NOT NULL,
		user2_role VARCHAR(10) NOT NULL,
		last_message TEXT,
		last_message_at TIMESTAMP NULL DEFAULT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE KEY uk_users (user1_id, user1_role, user2_id, user2_role)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// 聊天消息表：每条消息一条记录
		`CREATE TABLE IF NOT EXISTS chat_messages (
		message_id INT AUTO_INCREMENT PRIMARY KEY,
		conversation_id INT NOT NULL,
		sender_id INT NOT NULL,
		sender_role VARCHAR(10) NOT NULL,
		receiver_id INT NOT NULL,
		receiver_role VARCHAR(10) NOT NULL,
		content TEXT NOT NULL,
		read_at TIMESTAMP NULL DEFAULT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_conversation (conversation_id),
		INDEX idx_receiver_unread (receiver_id, receiver_role, read_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, s := range schema {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("执行建表语句失败: %w", err)
		}
	}
	log.Println("[DB] 数据表初始化完成")      //[DB] 方便区分是哪个模块的日志
	//fmt.Println：只打印文字，不带时间
	//log.Println：自带系统时间，排查启动问题很方便
	//log.Fatal() / log.Fatalln() :打印日志 + 直接退出程序 (os.Exit (1))
	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
