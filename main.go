package main

import (
	"breathaipay/database"
	"breathaipay/mail"
	"breathaipay/openwebui"
	"breathaipay/utils"

	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/paymentintent"
)

// UserInfo 结构体用于存储用户信息

// Order 结构体用于存储订单信息
type Order struct {
	ID        int64  `json:"id"`
	OrderID   string `json:"order_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"` // 添加过期时间字段
}

type Product struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Price  float64 `json:"price"`
	Points int     `json:"points"`
}

// 从函数获取商品列表
func GetProducts() []Product {
	return []Product{
		{ID: 1, Name: "100,000 积分", Price: 20.0, Points: 100000},
		{ID: 2, Name: "500,000 积分", Price: 50.0, Points: 500000},
		{ID: 3, Name: "1,000,000 积分", Price: 100.0, Points: 1000000},
	}
}

func main() {
	// 初始化数据库
	database.InitDB()
	defer database.CloseDb() // 结束后关闭数据库连接

	// 初始化Stripe - 正确设置API密钥
	stripe.Key = utils.GetEnvVariable("STRIPE_PRIVATE_KEY", "")
	pubKey := utils.GetEnvVariable("STRIPE_PUBLIC_KEY", "")
	if stripe.Key == "" || pubKey == "" {
		log.Fatal("没有配置Stripe密钥")
	}

	// 设置时区
	time.Local, _ = time.LoadLocation("Asia/Shanghai")

	// 获取调试模式
	debugMode := utils.GetEnvVariable("DEBUG_MODE", "true")

	// 是否信任所有代理
	trustProxies := utils.GetEnvVariable("TRUST_ALL_PROXIES", "false")

	if debugMode != "true" { // 放在新建Gin实例前
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 添加日志和恢复中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 创建自定义模板函数
	funcMap := template.FuncMap{
		"parseFloat": func(s string) float64 {
			val, _ := strconv.ParseFloat(s, 64)
			return val
		},
		"parseInt": func(s string) int {
			val, _ := strconv.Atoi(s)
			return val
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
		"printf": fmt.Sprintf,
	}

	// 应用模板函数
	r.SetFuncMap(funcMap)

	// 设置HTML模板目录
	r.LoadHTMLGlob("templates/*")

	// 提供静态文件服务
	r.Static("/static", "./static")

	// 首页 - 商品选择页面
	r.GET("/", func(c *gin.Context) {
		products := GetProducts()
		c.HTML(http.StatusOK, "product.html", gin.H{"products": products})
	})

	// 信息填写页面
	r.POST("/checkout", func(c *gin.Context) {
		productIDStr := c.PostForm("productID")
		productID, err := strconv.Atoi(productIDStr)
		if err != nil {
			log.Printf("商品ID转换失败: %v", err)
			c.HTML(http.StatusOK, "product.html", nil)
			return
		}

		// 根据商品ID获取商品信息
		products := GetProducts()
		var selectedProduct *Product
		for _, p := range products {
			if p.ID == productID {
				selectedProduct = &p
				break
			}
		}

		if selectedProduct == nil {
			log.Printf("未找到ID为 %d 的商品", productID)
			c.HTML(http.StatusOK, "product.html", nil)
			return
		}

		c.HTML(http.StatusOK, "checkout.html", gin.H{
			"Points":    selectedProduct.Name,
			"Price":     fmt.Sprintf("%.0f", selectedProduct.Price),
			"ProductID": selectedProduct.ID,
		})
	})

	// 付款页面
	r.POST("/payment", func(c *gin.Context) {
		productIDStr := c.PostForm("productID")
		siteType := c.PostForm("siteType")
		quantityStr := c.PostForm("quantity")
		email := c.PostForm("email")

		// 验证商品ID
		productID, err := strconv.Atoi(productIDStr)
		if err != nil {
			log.Printf("商品ID转换失败: %v", err)
			c.HTML(http.StatusOK, "product.html", nil)
			return
		}

		// 根据商品ID重新获取商品信息，防止篡改
		products := GetProducts()
		var selectedProduct *Product
		for _, p := range products {
			if p.ID == productID {
				selectedProduct = &p
				break
			}
		}

		if selectedProduct == nil {
			log.Printf("未找到ID为 %d 的商品", productID)
			c.HTML(http.StatusOK, "product.html", nil)
			return
		}

		// 使用从后端获取的真实价格，而不是前端传来的值
		price := int(selectedProduct.Price)

		// 验证quantity是否为有效值
		quantityVal, err := strconv.Atoi(quantityStr)
		if err != nil || quantityVal < 1 {
			log.Printf("购买数量无效: %v", err)
			c.HTML(http.StatusOK, "product.html", nil)
			return
		}

		// 计算包含手续费的总价，使用后端的价格
		total := (float64(price*quantityVal) + 1.9) / 0.971

		c.HTML(http.StatusOK, "payment.html", gin.H{
			"ProductID":         productID,
			"Points":            selectedProduct.Name,
			"Price":             fmt.Sprintf("%.0f", selectedProduct.Price),
			"SiteType":          siteType,
			"Quantity":          quantityStr, // 保持为字符串以满足模板显示需求
			"Email":             email,
			"Total":             total,
			"STRIPE_PUBLIC_KEY": pubKey,
		})
	})

	r.POST("/api/payment", createPaymentIntent) // 修改为创建PaymentIntent

	// 替换原有的success路由处理器
	r.GET("/success", successPageHandler)

	// 启动定期清理过期订单的goroutine
	go func() {
		for {
			// log.Println("开始删除过期订单")
			err := database.DeleteExpiredOrder()
			if err != nil {
				log.Printf("删除过期订单出错: %v", err)
			}
			time.Sleep(time.Second * 10) // 10s 删除一次
		}
	}()

	if trustProxies != "true" {
		// 设置信任本机/Docker网络代理
		r.SetTrustedProxies([]string{"127.0.0.1", "172.16.0.0/12"})
	}

	r.Run(":4399")

}

func createPaymentIntent(c *gin.Context) {

	// 计算过期时间 - 30分钟后过期
	expiresAt := time.Now().Add(30 * time.Minute)

	// --- 1. 解析请求体 (如果需要接收来自前端的数据，如商品ID等) ---
	productIDStr := c.PostForm("productID")
	// price := c.PostForm("price") // 保留这个参数用于后向兼容
	siteType := c.PostForm("siteType")
	quantityStr := c.PostForm("quantity") // 购买数量
	email := c.PostForm("email")

	// 验证商品ID
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		log.Printf("商品ID转换失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "无效的商品ID",
			},
		})
		return
	}

	// 根据商品ID重新获取商品信息，防止篡改
	products := GetProducts()
	var selectedProduct *Product
	for _, p := range products {
		if p.ID == productID {
			selectedProduct = &p
			break
		}
	}

	if selectedProduct == nil {
		log.Printf("未找到ID为 %d 的商品", productID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "无效的商品ID",
			},
		})
		return
	}

	// 验证quantity是否为有效值
	quantityVal, err := strconv.Atoi(quantityStr)
	if err != nil || quantityVal < 1 {
		log.Printf("购买数量无效: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "无效的购买数量",
			},
		})
		return
	}

	// 使用从后端获取的真实价格，而不是前端传来的价格参数
	priceVal := int(selectedProduct.Price)
	total := (float64(priceVal*quantityVal) + 1.9) / 0.971

	// 获取客户ID
	customerId, err := database.GetCustomerId(email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "创建客户失败",
			},
		})
		return
	}

	// --- 2. 准备 PaymentIntent 参数 ---
	params := &stripe.PaymentIntentParams{
		Amount:       stripe.Int64(int64(total * 100.0)),
		Currency:     stripe.String(string(stripe.CurrencyCNY)),
		Description:  stripe.String("购买灵息积分"),
		ReceiptEmail: stripe.String(email),
		Metadata: map[string]string{ // 添加元数据
			"email":     email,
			"sitetype":  siteType,
			"amount":    strconv.Itoa(selectedProduct.Points * quantityVal), // 使用后端给出的积分数量
			"productID": strconv.Itoa(productID),                            // 记录商品ID到元数据
		},
		// 启用自动支付方式选择
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled:        stripe.Bool(true),
			AllowRedirects: stripe.String("always"),
		},
		Customer: stripe.String(customerId),
	}

	// --- 3. 调用 Stripe API 创建 PaymentIntent ---
	pi, err := paymentintent.New(params)
	if err != nil {
		// --- 4. 处理 Stripe API 错误 ---
		log.Printf("Stripe API error: %v\n", err) // 记录详细错误到服务器日志

		// 检查是否是amount_too_large错误
		if stripeErr, ok := err.(*stripe.Error); ok {
			if stripeErr.Code == "amount_too_large" {
				// 向前端返回特定的错误信息
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "订单金额过大，请减少购买数量或选择其他商品。",
						"code":    "amount_too_large",
					},
				})
				return
			}
		}

		// 向前端返回通用的用户友好错误信息
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "创建失败，请稍后再试。", // 或者返回 err.Error() (不推荐直接暴露给前端)
			},
		})
		return
	}

	// 记录订单到数据库，包含过期时间
	err = database.RecordOrder(pi.ID, "created", expiresAt)
	if err != nil {
		log.Printf("记录订单到数据库失败 (%s): %v", pi.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "系统错误，无法记录订单，请联系客服。",
			},
		})
		return
	}

	// --- 5. 成功创建，返回 client_secret ---
	log.Printf("PaymentIntent created: %s\n", pi.ID) // 记录日志
	c.JSON(http.StatusOK, gin.H{
		"clientSecret": pi.ClientSecret,
		"expiresAt":    expiresAt.Unix(), // 返回过期时间戳
	})
}

func successPageHandler(c *gin.Context) {
	// 1. 从查询参数中获取 PaymentIntent ID
	paymentIntentID := c.Query("payment_intent")
	// clientSecret := c.Query("payment_intent_client_secret") // 有时也会用到，用于额外验证
	redirectStatus := c.Query("redirect_status") // "succeeded"

	if redirectStatus != "succeeded" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid payment intent ID",
		})
	}

	// 2. 基本检查
	if paymentIntentID == "" {
		c.String(http.StatusBadRequest, "缺少 Payment Intent ID")
		return
	}
	// 可以记录 redirectStatus 日志，但不要完全依赖它作为成功依据

	// 3. 调用 Stripe API 获取 PaymentIntent 详情
	// 确保 stripe.Key 已经在 main 函数中用你的 Secret Key 初始化
	pi, err := paymentintent.Get(paymentIntentID, nil) // 第二个参数是可选的请求参数
	if err != nil {
		log.Printf("获取 PaymentIntent 失败 (%s): %v", paymentIntentID, err)
		// 可能是 ID 无效，或网络问题等
		c.String(http.StatusInternalServerError, "无法验证支付状态，请联系客服。")
		return
	}

	// 4. 验证 PaymentIntent 状态和其他关键信息
	switch pi.Status {
	case stripe.PaymentIntentStatusSucceeded:
		// 支付成功
		log.Printf("支付成功: PaymentIntent ID=%s, Amount=%d, Currency=%s", pi.ID, pi.Amount, pi.Currency)

		// 检查订单是否已经处理过，防止重复处理
		alreadyProcessed, err := database.IsOrderProcessed(paymentIntentID)
		if err != nil {
			log.Printf("检查订单是否已处理时出错 (%s): %v", paymentIntentID, err)
			c.String(http.StatusInternalServerError, "系统错误，请联系客服。")
			return
		}

		if alreadyProcessed {
			log.Printf("订单已处理过，跳过重复处理: %s", paymentIntentID)
			c.HTML(http.StatusOK, "success.html", gin.H{
				"paymentIntentID": paymentIntentID,
				"amount":          pi.Amount / 100.0,
				"currency":        pi.Currency,
				"email":           "", // 订单已处理，但没有获取到邮箱
				"sitetype":        "", // 订单已处理，但没有获取到站点类型
			})
			return
		}

		// 5. 获取并处理你的业务订单信息
		var email string
		var siteType string
		var realAmount int
		if pi.Metadata != nil {
			email = pi.Metadata["email"]
			siteType = pi.Metadata["sitetype"]
			realAmount, _ = strconv.Atoi(pi.Metadata["amount"])
		}

		// 更新订单状态为已成功
		err = database.UpdateOrderStatus(paymentIntentID, "succeeded", false)
		if err != nil {
			log.Printf("更新订单状态到数据库失败 (%s): %v", paymentIntentID, err)
			c.String(http.StatusInternalServerError, "系统错误，无法记录订单，请联系客服。")
			return
		}

		log.Printf("Real Amount: %d", realAmount)
		switch siteType {
		case "international":
			go finishPay(email, int64(realAmount), 1)
		case "domestic":
			go finishPay(email, int64(realAmount), 2)
		}

		// 6. 向用户返回成功页面
		c.HTML(http.StatusOK, "success.html", gin.H{
			"paymentIntentID": paymentIntentID,
			"amount":          pi.Amount / 100.0,
			"currency":        pi.Currency,
			"email":           email,
			"sitetype":        siteType,
		})

	case stripe.PaymentIntentStatusCanceled, stripe.PaymentIntentStatusRequiresPaymentMethod:
		// 支付失败或需要其他支付方式
		log.Printf("支付失败或已取消: PaymentIntent ID=%s, Status=%s", pi.ID, pi.Status)
		c.String(http.StatusBadRequest, "支付失败或已取消。")
	default:
		log.Printf("未知支付状态: PaymentIntent ID=%s, Status=%s", pi.ID, pi.Status)
		c.String(http.StatusInternalServerError, "未知支付状态。")
	}
}

// 处理积分增加

func finishPay(email string, amount int64, sitetype int) {
	user := openwebui.GetUserIDWithEmail(email, sitetype)
	err := openwebui.AddBalance(user, amount, sitetype)
	if err != nil {
		log.Println("Failed to add balance:", err)
		return
	} else {
		log.Print("处理完成, 发送确认邮件")
		mail.NewMailer().SendMail([]string{email}, "积分已到账", fmt.Sprintf("您好,尊敬的灵息用户 %s , 您的 %s 积分已到账<br><br>灵息.com 自动邮件<br>请勿回复", email, strconv.Itoa(int(amount))), "text/html")
	}
}
