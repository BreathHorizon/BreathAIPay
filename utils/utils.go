package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GetEnvVariable 从 .env 文件或系统环境中获取环境变量
// 首先尝试从 .env 文件中读取，如果文件中没有则从系统环境变量中获取
func GetEnvVariable(key string, defaultValue string) string {
	// 尝试从 .env 文件中获取
	envValue := getEnvFromDotEnv(key)
	if envValue != "" {
		return envValue
	}

	// 如果 .env 文件中没有，则从系统环境变量中获取
	sysValue := os.Getenv(key)
	if sysValue != "" {
		return sysValue
	}

	// 如果都没有，则返回默认值
	return defaultValue
}

// getEnvFromDotEnv 从 .env 文件中获取指定的环境变量
func getEnvFromDotEnv(key string) string {
	file, err := os.Open(".env")
	if err != nil {
		// 如果文件不存在，记录但不返回错误，因为文件可能不存在
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过注释行和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式的行
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envKey := strings.TrimSpace(parts[0])
		envValue := strings.TrimSpace(parts[1])

		// 去除值两边的引号（如果有的话）
		if (strings.HasPrefix(envValue, "\"") && strings.HasSuffix(envValue, "\"")) ||
			(strings.HasPrefix(envValue, "'") && strings.HasSuffix(envValue, "'")) {
			envValue = envValue[1 : len(envValue)-1]
		}

		if envKey == key {
			return envValue
		}
	}

	return ""
}

// LoadEnvFile 加载整个 .env 文件到环境变量中
func LoadEnvFile() error {
	file, err := os.Open(".env")
	if err != nil {
		return fmt.Errorf("无法打开 .env 文件: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过注释行和空行
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// 解析 KEY=VALUE 格式的行
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envKey := strings.TrimSpace(parts[0])
		envValue := strings.TrimSpace(parts[1])

		// 去除值两边的引号（如果有的话）
		if (strings.HasPrefix(envValue, "\"") && strings.HasSuffix(envValue, "\"")) ||
			(strings.HasPrefix(envValue, "'") && strings.HasSuffix(envValue, "'")) {
			envValue = envValue[1 : len(envValue)-1]
		}

		// 设置环境变量，但仅当系统环境变量中不存在此键时
		if os.Getenv(envKey) == "" {
			os.Setenv(envKey, envValue)
		}
	}

	return scanner.Err()
}
