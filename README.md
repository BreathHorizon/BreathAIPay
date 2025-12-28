# BreathAIPay
连接Stripe和OpenWebUI的桥梁 - 自动化支付&增加积分

***

### 项目启动
```bash
go run main.go
```
项目监听4399端口  

### 环境变量
环境变量可以存于项目根目录的`.env`文件或者写在系统环境变量中, 程序会优先查找`.env`中的变量
> 标*为必选, <del>删除线</del>为已弃用
| 变量名 | 用途 |
| :--: | :--: |
| DEBUG_MODE | 设置是否为调试模式(除了true外均为非调试) |
| SMTP_HOST | SMTP协议主机 |
| SMTP_USERNAME | SMTP登录名 |
| SMTP_PORT | SMTP登录端口 |
| SMTP_PASSWORD | SMTP登录密码 |
| OPENWEBUI_INTERNATIONAL_TOKEN * | 国际站JWT Token |
| <del>OPENWEBUI_CHINESE_TOKEN</del> | <del>中国站JWT Token</del> |
| STRIPE_PRIVATE_KEY * | Stripe私钥 |
| STRIPE_PUBLIC_KEY * | Stripe公钥 |
> 警告: 如果不配置Stripe公/私钥, 程序将无法启动

### 数据库说明
项目使用Sqlite数据库, 会在根目录下**自动**建立`customers.db`和`orders.db`, 请确保程序有足够的写入权限

### 额外说明
- 项目不依赖静态CDN服务, 而是采用本地服务器的js/css文件