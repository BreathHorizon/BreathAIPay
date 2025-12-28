package mail

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"unicode/utf8"

	"breathaipay/utils"
)

// Mailer 邮件发送器结构体
type Mailer struct {
	Host     string
	Port     string
	Username string
	Password string
}

// NewMailer 创建新的邮件发送器实例
func NewMailer() *Mailer {
	return &Mailer{
		Host:     utils.GetEnvVariable("SMTP_HOST", "smtp.gmail.com"),
		Port:     utils.GetEnvVariable("SMTP_PORT", "587"),
		Username: utils.GetEnvVariable("SMTP_USERNAME", ""),
		Password: utils.GetEnvVariable("SMTP_PASSWORD", ""),
	}
}

// SendMail 发送邮件
func (m *Mailer) SendMail(to []string, subject, body, contentType string) error {
	if m.Username == "" || m.Password == "" {
		return fmt.Errorf("SMTP credentials are not set")
	}

	if len(to) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	// 验证收件人邮箱格式
	for _, recipient := range to {
		if !isValidEmail(recipient) {
			return fmt.Errorf("invalid email address: %s", recipient)
		}
	}

	// 设置邮件头
	headers := make(map[string]string)
	headers["From"] = m.Username
	headers["To"] = strings.Join(to, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"

	// 设置内容类型
	if contentType == "" {
		contentType = "text/plain"
	}
	headers["Content-Type"] = fmt.Sprintf("%s; charset=UTF-8", contentType)

	// 组装邮件头
	message := ""
	for key, value := range headers {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	message += "\r\n" + body

	// 创建SMTP认证
	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)

	// 连接SMTP服务器
	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)

	// 使用STARTTLS发送邮件
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// 启用STARTTLS
	if err = conn.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// 设置发件人
	if err = conn.Mail(m.Username); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// 设置收件人
	for _, recipient := range to {
		if err = conn.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// 发送邮件内容
	writer, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	defer writer.Close()

	// 使用auth变量进行认证
	if err = conn.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	_, err = writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	if err = writer.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	log.Print("邮件发送成功")
	return conn.Quit()
}

// isValidEmail 验证邮箱格式
func isValidEmail(email string) bool {
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]
	if local == "" || domain == "" || !utf8.ValidString(email) {
		return false
	}

	return true
}
