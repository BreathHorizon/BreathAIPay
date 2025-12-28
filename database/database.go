package database

import (
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/customer"
	"github.com/stripe/stripe-go/v84/paymentintent"

	_ "modernc.org/sqlite"
)

var db *sql.DB                  // 数据库连接器
var dbMutex sync.Mutex          // 数据库操作锁，用于解决并发访问问题
var dbMutexCustomers sync.Mutex // 客户数据库操作锁，用于解决并发访问问题

var dbCustomers *sql.DB

func InitDB() error {
	var err error
	db, err = sql.Open("sqlite", "./orders.db?cache=shared&journal_mode=WAL")
	if err != nil {
		log.Fatal("打开数据库失败:", err)
		return err
	}

	dbCustomers, err = sql.Open("sqlite", "./custormers.db?cache=shared&journal_mode=WAL")
	if err != nil {
		log.Fatal("打开数据库失败:", err)
		return err
	}

	// 设置连接池参数，减少锁竞争
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	dbCustomers.SetMaxOpenConns(1)
	dbCustomers.SetMaxIdleConns(1)

	// 创建订单表
	sqlTable := `CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL
	);`

	_, err = db.Exec(sqlTable)
	if err != nil {
		log.Fatal("创建订单表失败:", err)
		return err
	}

	// 客户记录表
	sqlTable = `CREATE TABLE IF NOT EXISTS customers (
		id TEXT PRIMARY KEY,
		mail TEXT NOT NULL UNIQUE
	)`
	_, err = dbCustomers.Exec(sqlTable)
	if err != nil {
		log.Fatal("创建客户表失败:", err)
		return err
	}

	log.Println("数据库初始化成功")
	return nil
}

func CloseDb() {
	db.Close()
}

func UpdateOrderStatus(orderID string, status string, ignoreLock bool) error {
	if !ignoreLock { // 在DeleteExpiredOrder中, 会持有锁对象, 如果这里不忽略锁, 则会造成死锁
		dbMutex.Lock()
		defer dbMutex.Unlock()
	} else {
		log.Print("跳过锁检查")
	}
	query := "UPDATE orders SET status = ? WHERE order_id = ?"
	_, err := db.Exec(query, status, orderID)
	log.Print("SQL执行完成")
	return err
}

func IsOrderProcessed(orderID string) (bool, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	var count int
	query := "SELECT COUNT(*) FROM orders WHERE order_id = ? AND status = 'succeeded'"
	err := db.QueryRow(query, orderID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func RecordOrder(orderID string, status string, expiresAt time.Time) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	query := "INSERT INTO orders (order_id, status, expires_at) VALUES (?, ?, ?)"
	_, err := db.Exec(query, orderID, status, expiresAt.Format("2006-01-02 15:04:05"))
	return err
}

func DeleteExpiredOrder() error {
	dbMutex.Lock()
	// log.Printf("开始查询过期订单...")

	// 收集所有需要处理的过期订单ID
	var expiredOrderIDs []string
	nowTime := time.Now().Format("2006-01-02 15:04:05")
	query := "SELECT order_id FROM orders WHERE status = 'created' AND expires_at < ?"

	// 先执行查询获取所有过期订单ID
	rows, err := db.Query(query, nowTime)
	if err != nil {
		log.Printf("查询过期订单失败: %v", err)
		dbMutex.Unlock()
		return err
	}

	for rows.Next() {
		var orderID string
		err := rows.Scan(&orderID)
		if err != nil {
			log.Printf("扫描订单记录出错: %v", err)
			continue
		}
		expiredOrderIDs = append(expiredOrderIDs, orderID)
	}

	// 检查扫描过程是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("扫描订单行时出错: %v", err)
		rows.Close()
		dbMutex.Unlock()
		return err
	}

	rows.Close()
	// 在处理订单前释放锁，避免死锁
	dbMutex.Unlock()
	if len(expiredOrderIDs) > 0 {
		log.Printf("查询完成，共找到 %d 个过期订单", len(expiredOrderIDs))
	}

	// 处理每个过期订单，此时不再持有全局锁
	count := 0
	for _, orderID := range expiredOrderIDs {
		count++
		log.Printf("第 %d 个 - 发现过期订单: %s", count, orderID)

		// 首先获取支付意图以检查其状态
		pi, err := paymentintent.Get(orderID, nil)
		if err != nil {
			log.Printf("获取支付意图失败 %s: %v", orderID, err)
			// 如果获取失败，仍然更新数据库状态
			if err := UpdateOrderStatus(orderID, "error_retrieving", false); err != nil {
				log.Printf("更新订单状态失败 %s: %v", orderID, err)
			}
			continue
		}

		// 检查支付意图是否可以取消
		if pi.Status == stripe.PaymentIntentStatusCanceled ||
			pi.Status == stripe.PaymentIntentStatusSucceeded ||
			pi.Status == stripe.PaymentIntentStatusRequiresCapture {
			log.Printf("支付意图 %s 状态为 %s，跳过取消操作", orderID, pi.Status)
			// 根据状态更新本地数据库
			var newStatus string
			switch pi.Status {
			case stripe.PaymentIntentStatusCanceled:
				newStatus = "canceled"
			case stripe.PaymentIntentStatusSucceeded:
				newStatus = "succeeded"
			case stripe.PaymentIntentStatusRequiresCapture:
				newStatus = "requires_capture"
			}
			if err := UpdateOrderStatus(orderID, newStatus, false); err != nil {
				log.Printf("更新订单状态失败 %s: %v", orderID, err)
			}
			log.Printf("第 %d 个订单处理完成", count)
			continue
		}

		// 使用Stripe支持的取消原因 - 改为"abandoned"（已放弃）
		params := &stripe.PaymentIntentCancelParams{
			CancellationReason: stripe.String("abandoned"),
		}

		log.Printf("正在取消订单: %s", orderID)
		_, err = paymentintent.Cancel(orderID, params)
		log.Printf("取消订单API调用完成，订单ID: %s", orderID)

		if err != nil {
			// 如果取消失败，记录错误但继续处理其他订单
			log.Printf("取消过期订单失败 %s: %v", orderID, err)
			// 即使取消API调用失败，也要更新数据库状态
			if err := UpdateOrderStatus(orderID, "canceled_due_to_error", false); err != nil {
				log.Printf("更新订单状态失败 %s: %v", orderID, err)
			}
			log.Printf("第 %d 个订单处理完成（取消失败）", count)
			continue
		}

		if err := UpdateOrderStatus(orderID, "canceled", false); err != nil {
			log.Printf("更新订单状态失败 %s: %v", orderID, err)
			log.Printf("第 %d 个订单处理完成（更新状态失败）", count)
			continue
		}
		log.Printf("订单 %s 已取消", orderID)
		// log.Printf("第 %d 个订单处理完成", count)
	}
	if count > 0 {
		log.Printf("本次共处理了 %d 个过期订单", count)
	}
	// log.Printf("完成过期订单处理")
	return nil
}

func GetCustomerId(email string) (string, error) {
	// 获取CustomerID , 如果不存在则新建
	dbMutexCustomers.Lock()
	var id string
	sqlQuery := "SELECT id FROM customers WHERE mail = ?"
	err := dbCustomers.QueryRow(sqlQuery, email).Scan(&id)
	dbMutexCustomers.Unlock()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Print("客户: ", email, "不存在, 开始新建")
		} else {
			return "", err
		}
	} else {
		return id, nil
	}
	// 调用Stripe创建新客户
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	customerObj, err := customer.New(params)
	if err != nil {
		return "", err
	}
	// 写入数据库
	dbMutexCustomers.Lock()
	sqlQuery = "INSERT INTO customers (id, mail) VALUES (?, ?)"
	_, err = dbCustomers.Exec(sqlQuery, customerObj.ID, email) // 写入数据库
	dbMutexCustomers.Unlock()
	if err != nil {
		return customerObj.ID, err
	}
	return customerObj.ID, nil
}
