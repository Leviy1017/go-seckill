// MySQL 数据操作层（DAO 层），封装所有增删改查 SQL，专门操作四张表：买家、卖家、秒杀商品、订单。
// queries = 数据库查询操作  包含了所有与数据库交互的函数
// 封装了所有对 MySQL 的 CRUD 操作，让上层 Handler 不用写 SQL
package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"go-seckill/models"
)

// ============================== 买家相关 ==============================

func CreateBuyer(username, password, phone, address string) (*models.Buyer, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	//密码加密 []byte(password)：字符串转字节数组，函数要求参数格式
	//bcrypt.GenerateFromPassword()：第三方加密函数    bcrypt.DefaultCost：加密复杂度（默认档次）
	//hashed：加密后的一长串乱码字符串，类似：$2a$10$xxxxxxx
	if err != nil {
		return nil, err
	}

	result, err := DB.Exec(
		`INSERT INTO buyers (username, password, phone, address) VALUES (?, ?, ?, ?)`,
		username, hashed, phone, address,
	)
	//Exec：(execute 执行)增、删、改 → result
	//QueryRow：查 1 条 → .Scan 赋值到结构体
	//Query：查多条 → rows 循环遍历
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	//.LastInsertId()：库自带方法，返回本次新增记录的自增 ID   只有 INSERT（Exec 新增）能用
	if err != nil {
		return nil, err
	}
	return GetBuyerByID(int(id))
}

func GetBuyerByPhone(phone string) (*models.Buyer, error) {
	var b models.Buyer
	err := DB.QueryRow(
		`SELECT buyer_id, username, password, phone, address, created_at, updated_at FROM buyers WHERE phone = ?`,
		phone, //phone填充?
	).Scan(&b.BuyerID, &b.Username, &b.Password, &b.Phone, &b.Address, &b.CreatedAt, &b.UpdatedAt)
	//SQL查出几列，Scan就传几个&地址，顺序严格对应select字段
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func GetBuyerByID(id int) (*models.Buyer, error) {
	var b models.Buyer
	err := DB.QueryRow(
		`SELECT buyer_id, username, password, phone, address, created_at, updated_at FROM buyers WHERE buyer_id = ?`,
		id,
	).Scan(&b.BuyerID, &b.Username, &b.Password, &b.Phone, &b.Address, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ============================== 卖家相关 ==============================

func CreateSeller(shopName, password, phone, shopAddr string) (*models.Seller, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	result, err := DB.Exec(
		`INSERT INTO sellers (shop_name, password, phone, shop_addr) VALUES (?, ?, ?, ?)`,
		shopName, hashed, phone, shopAddr,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return GetSellerByID(int(id))
}

func GetSellerByPhone(phone string) (*models.Seller, error) {
	var s models.Seller
	err := DB.QueryRow(
		`SELECT seller_id, shop_name, password, phone, shop_addr, created_at, updated_at FROM sellers WHERE phone = ?`,
		phone,
	).Scan(&s.SellerID, &s.ShopName, &s.Password, &s.Phone, &s.ShopAddr, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func GetSellerByID(id int) (*models.Seller, error) {
	var s models.Seller
	err := DB.QueryRow(
		`SELECT seller_id, shop_name, password, phone, shop_addr, created_at, updated_at FROM sellers WHERE seller_id = ?`,
		id,
	).Scan(&s.SellerID, &s.ShopName, &s.Password, &s.Phone, &s.ShopAddr, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ============================== 秒杀商品相关 ==============================

func CreateSeckillProduct(p *models.SeckillProduct) error {
	result, err := DB.Exec(
		`INSERT INTO seckill_products (seller_id, name, description, original_price, seckill_price, stock, seckill_start, seckill_end, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		p.SellerID, p.Name, p.Description, p.OriginalPrice, p.SeckillPrice,
		p.Stock, p.SeckillStart, p.SeckillEnd,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ProductID = int(id)
	// 取回完整记录（填充 CreatedAt/UpdatedAt）
	created, err := GetSeckillProductByID(p.ProductID)
	if err != nil {
		return err
	}
	p.CreatedAt = created.CreatedAt
	p.UpdatedAt = created.UpdatedAt
	return nil
}

// 查询全部商品列表
func GetSeckillProducts() ([]models.SeckillProduct, error) {
	now := time.Now()
	// 自动更新状态
	DB.Exec(`UPDATE seckill_products SET status = 'active' WHERE status = 'pending' AND seckill_start <= ? AND seckill_end > ?`, now, now)
	DB.Exec(`UPDATE seckill_products SET status = 'ended' WHERE status = 'active' AND seckill_end <= ?`, now)

	rows, err := DB.Query(
		`SELECT product_id, seller_id, name, description, original_price, seckill_price, stock, seckill_start, seckill_end, status, created_at, updated_at
		 FROM seckill_products ORDER BY seckill_start ASC`,
	)
	//QueryRow 只能查 1 条，Query 是一堆多行数据，没法链式.Scan
	if err != nil {
		return nil, err
	}
	defer rows.Close() //用完关闭游标（必写，防止数据库泄漏连接）

	var products []models.SeckillProduct
	for rows.Next() { //循环每一行，rows.Scan装进结构体，append放进切片返回
		var p models.SeckillProduct
		rows.Scan(&p.ProductID, &p.SellerID, &p.Name, &p.Description,
			&p.OriginalPrice, &p.SeckillPrice, &p.Stock,
			&p.SeckillStart, &p.SeckillEnd, &p.Status, &p.CreatedAt, &p.UpdatedAt)
		products = append(products, p)
	}
	// 遍历结束后检查是否在中途发生错误（网络断开、超时等）
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func GetSeckillProductByID(productID int) (*models.SeckillProduct, error) {
	var p models.SeckillProduct
	err := DB.QueryRow(
		`SELECT product_id, seller_id, name, description, original_price, seckill_price, stock, seckill_start, seckill_end, status, created_at, updated_at
		 FROM seckill_products WHERE product_id = ?`,
		productID,
	).Scan(&p.ProductID, &p.SellerID, &p.Name, &p.Description,
		&p.OriginalPrice, &p.SeckillPrice, &p.Stock,
		&p.SeckillStart, &p.SeckillEnd, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func UpdateStock(productID int, newStock int) error {
	_, err := DB.Exec(`UPDATE seckill_products SET stock = ? WHERE product_id = ?`, newStock, productID)
	return err
}

// ============================== 订单相关 ==============================

// 下单 + 扣库存，全部放在一个数据库事务里
// 保证：库存扣了订单一定创建了，或者都没发生
func ProcessSeckillOrder(buyerID, productID int, productName string, seckillPrice float64, buyerName, sellerName string, sellerID int) (*models.SeckillOrder, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() // 出问题就回滚

	//更新日志：防重下单仅依赖 Redis SETNX，缺乏数据库层兜底
	// 1. 数据库层防重：同一买家同一商品已有订单就不让再下
	var dupCount int
	tx.QueryRow(
		`SELECT COUNT(*) FROM seckill_orders WHERE buyer_id = ? AND product_id = ?`,
		buyerID, productID,
	).Scan(&dupCount)
	if dupCount > 0 {
		return nil, fmt.Errorf("重复下单")
	}

	// 2. 扣库存（行级锁兜底）
	result, err := tx.Exec(
		`UPDATE seckill_products SET stock = stock - 1, updated_at = NOW()
		 WHERE product_id = ? AND stock > 0`, productID)
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("已售罄")
	}

	// 3. 创建订单
	orderID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO seckill_orders (order_id, buyer_id, seller_id, product_id, product_name, seckill_price, order_status, buyer_name, seller_name)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		orderID, buyerID, sellerID, productID, productName, seckillPrice,
		models.OrderStatusPaid, buyerName, sellerName,
	)
	if err != nil {
		return nil, err
	}

	// 4. 检查库存是否清零，是的话更新状态
	tx.Exec(`UPDATE seckill_products SET status='sold_out' WHERE product_id=? AND stock=0`, productID)

	// 5. 提交事务
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetOrderByID(orderID)
}

func GetOrderByID(orderID string) (*models.SeckillOrder, error) {
	var o models.SeckillOrder
	err := DB.QueryRow(
		`SELECT order_id, buyer_id, seller_id, product_id, product_name, seckill_price, order_status, buyer_name, seller_name, order_time, created_at, updated_at
		 FROM seckill_orders WHERE order_id = ?`,
		orderID,
	).Scan(&o.OrderID, &o.BuyerID, &o.SellerID, &o.ProductID, &o.ProductName,
		&o.SeckillPrice, &o.OrderStatus, &o.BuyerName, &o.SellerName,
		&o.OrderTime, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func UpdateOrderStatus(orderID, status string) error {
	_, err := DB.Exec(`UPDATE seckill_orders SET order_status = ? WHERE order_id = ?`, status, orderID)
	return err
}

func GetSellerOrders(sellerID int) ([]models.SeckillOrder, error) {
	rows, err := DB.Query(
		`SELECT order_id, buyer_id, seller_id, product_id, product_name, seckill_price, order_status, buyer_name, seller_name, order_time, created_at, updated_at
		 FROM seckill_orders WHERE seller_id = ? ORDER BY order_time DESC`,
		sellerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.SeckillOrder
	for rows.Next() {
		var o models.SeckillOrder
		rows.Scan(&o.OrderID, &o.BuyerID, &o.SellerID, &o.ProductID, &o.ProductName,
			&o.SeckillPrice, &o.OrderStatus, &o.BuyerName, &o.SellerName,
			&o.OrderTime, &o.CreatedAt, &o.UpdatedAt)
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

func GetBuyerOrders(buyerID int) ([]models.SeckillOrder, error) {
	rows, err := DB.Query(
		`SELECT order_id, buyer_id, seller_id, product_id, product_name, seckill_price, order_status, buyer_name, seller_name, order_time, created_at, updated_at
		 FROM seckill_orders WHERE buyer_id = ? ORDER BY order_time DESC`,
		buyerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //归还连接

	var orders []models.SeckillOrder
	for rows.Next() {
		var o models.SeckillOrder
		rows.Scan(&o.OrderID, &o.BuyerID, &o.SellerID, &o.ProductID, &o.ProductName,
			&o.SeckillPrice, &o.OrderStatus, &o.BuyerName, &o.SellerName,
			&o.OrderTime, &o.CreatedAt, &o.UpdatedAt)
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

// ============================== 密码校验 ==============================

func CheckPassword(hashed, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}
