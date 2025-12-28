package openwebui

import (
	"breathaipay/utils"

	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type UserInfo struct {
	ID              string `json:"id"`
	Credit          int64  `json:"credit"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"`
	Role            string `json:"role"`
}

func AddBalance(userInfo UserInfo, amount int64, sitetype int) error {
	// 打印账户信息
	print("账户ID: ", userInfo.ID, " 余额: ", userInfo.Credit, " 昵称: ", userInfo.Name, " 邮箱: ", userInfo.Email, "\n")

	// 更新OpenWebUI的账户余额
	// 在原有余额基础上增加积分
	newCredit := userInfo.Credit + amount

	// 构造要更新的数据
	updateData := map[string]any{
		"credit":            newCredit,
		"name":              userInfo.Name,
		"email":             userInfo.Email,
		"profile_image_url": userInfo.ProfileImageURL,
		"role":              userInfo.Role,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(updateData)
	// print("\n更新数据: ", string(jsonData), "\n")
	if err != nil {
		return err
	}

	// 发起POST请求并携带请求头和数据
	var url string
	if sitetype == 1 { // 1为国际站
		url = "https://chat.breathai.top/api/v1/users/" + userInfo.ID + "/update"
	} else { // 2为中国站
		url = "https://breath.yearnstudio.cn/api/v1/users/" + userInfo.ID + "/update"
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json;encoding=utf-8")
	if sitetype == 1 { // 1为国际站
		req.Header.Set("Authorization", "Bearer "+utils.GetEnvVariable("OPENWEBUI_INTERNATIONAL_TOKEN", ""))
	} else { // 2为中国站
		req.Header.Set("Authorization", "Bearer "+utils.GetEnvVariable("OPENWEBUI_CHINESE_TOKEN", ""))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// fmt.Println(string(body))
	return nil
}

func GetUserIDWithEmail(email string, sitetype int) UserInfo {
	// 从OpenWebUI搜索中获取用户ID
	originalEmail := email // 保存原始邮箱用于返回
	// email = strings.ReplaceAll(email, "@", "%40") // 编码
	// 发起GET请求并携带请求头和param参数
	var url string
	if sitetype == 1 { // 1为国际站
		url = "https://chat.breathai.top/api/v1/users/?page=1&order_by=created_at&direction=asc&query=" + email
		// req, err := http.NewRequest("GET", "https://chat.breathai.top/api/v1/users/?page=1&query="+email, nil)
	} else { // 2为中国站
		url = "https://breath.yearnstudio.cn/api/v1/users/?page=1&order_by=created_at&direction=asc&query=" + email
		// req, err := http.NewRequest("GET", "https://openwebui.com/api/v1/users/search?email="+email, nil)
	}
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return UserInfo{} // 出错时返回空的UserInfo对象
	}
	req.Header.Set("Content-Type", "application/json;encoding=utf-8")
	if sitetype == 1 { // 1为国际站
		req.Header.Set("Authorization", "Bearer "+utils.GetEnvVariable("OPENWEBUI_INTERNATIONAL_TOKEN", ""))
	} else { // 2为中国站
		req.Header.Set("Authorization", "Bearer "+utils.GetEnvVariable("OPENWEBUI_CHINESE_TOKEN", ""))
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("执行请求失败: %v", err)
		return UserInfo{} // 出错时返回空的UserInfo对象
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, resp.Status)
		body, _ := io.ReadAll(resp.Body)
		log.Printf("错误响应体: %s", string(body))
		return UserInfo{} // 返回空的UserInfo对象
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应体失败: %v", err)
		return UserInfo{} // 出错时返回空的UserInfo对象
	}

	// 检查响应体是否以{或[开头，以确认是否为JSON
	responseStr := string(body)
	// fmt.Println("API响应:", responseStr)

	// 检查响应是否为有效的JSON格式
	if len(responseStr) == 0 {
		log.Println("响应体为空")
		return UserInfo{}
	}

	var result map[string]any
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Printf("解析JSON失败: %v, 响应内容: %s", err, responseStr)
		return UserInfo{} // 出错时返回空的UserInfo对象
	}
	// 判断users列表是否存在并遍历, 在每一项的email做完全匹配
	if result["users"] != nil {
		for _, v := range result["users"].([]any) {
			userData := v.(map[string]any)
			if userData["email"] == email {
				fmt.Println("存在")
				// 解析credit为int64
				var credit int64
				if creditVal, ok := userData["credit"]; ok {
					switch val := creditVal.(type) {
					case float64:
						credit = int64(val)
					case string:
						// 尝试解析字符串格式的数值
						if fVal, parseErr := strconv.ParseFloat(val, 64); parseErr == nil {
							credit = int64(fVal)
						} else if iVal, parseErr := strconv.ParseInt(val, 10, 64); parseErr == nil {
							credit = iVal
						} else {
							log.Printf("无法解析credit值: %s", val)
							credit = 0
						}
					case int64:
						credit = val
					case int:
						credit = int64(val)
					default:
						log.Printf("未知的credit类型: %T, 值: %v", creditVal, creditVal)
						credit = 0
					}
				}

				// 创建并返回UserInfo对象
				userInfo := UserInfo{
					ID:              getStringValue(userData, "id"),
					Credit:          credit,
					Email:           originalEmail,
					Name:            getStringValue(userData, "name"),
					ProfileImageURL: getStringValue(userData, "profile_image_url"),
					Role:            getStringValue(userData, "role"),
				}
				return userInfo
			}
		}
	}
	return UserInfo{} // 返回空的UserInfo对象
}

// getStringValue 从map中安全地获取字符串值
func getStringValue(data map[string]any, key string) string {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case float64:
			return fmt.Sprintf("%.0f", v) // 处理浮点数，转换为整数形式的字符串
		case int:
			return fmt.Sprintf("%d", v) // 处理整数
		case int64:
			return fmt.Sprintf("%d", v) // 处理int64
		default:
			return fmt.Sprintf("%v", v) // 处理其他类型
		}
	}
	return ""
}
