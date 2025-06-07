package env

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Signer 签名查询器
//
// 用于操作相同前缀但需要区分不同场景的环境变量时十分有用，比如
// 我们通过环境变量文件配置缓存配置
//
//	CACHE_DRIVER=redis
//	CACHE_DATABASE=1
//	CACHE_SCOPE=app:
//	CACHE_BOOK_DATABASE=10
//	CACHE_BOOK_SCOPE=app:books:
//
// 那么我们就可以十分方便的使用:
//
//	cache := env.Signed("CACHE", "BOOK")
//	cache.String("DRIVER") // redis
//	cache.Int("DATABASE")  // 10
//	cache.String("SCOPE")  // app:books:
//
// 这样就方便我们对环境变量简单分组分场景使用了。
type Signer interface {
	// Lookup 返回指定键的数据，只有存在指定的环境变量并且其值不为空时，
	// 第二个返回值为 true，其它情况下，均返回 false，与方法 Exists 有所区别。
	Lookup(key string) (string, bool)
	// Exists 判断指定键的数据是否存在
	// 只要存在键名就返回 true，不存在返回 false。
	Exists(key string) bool
	// String 返回指定键的数据的字符串形式，当数据不存在或值为空时返回默认值
	String(key string, fallback ...string) string
	// Bytes 返回指定键的数据的字节切片值，当数据不存在或值为空时返回默认值
	Bytes(key string, fallback ...[]byte) []byte
	// Int 返回指定键的数据的整数值，当数据不存在或值为空时返回默认值
	Int(key string, fallback ...int) int
	// Duration 返回指定键的数据的时长值，当数据不存在或值为空时返回默认值
	Duration(key string, fallback ...time.Duration) time.Duration
	// Bool 返回指定键的数据的布尔值，当数据不存在或值为空时返回默认值
	Bool(key string, fallback ...bool) bool
	// List 返回指定键的数据的字符串列表（使用英文逗号分割），当数据不存在或值为空时返回默认值
	List(key string, fallback ...[]string) []string
	// Map 将具体相同前缀的键的数据聚合起来返回
	Map(prefix string) map[string]string
	// Where 返回通过自定义函数过滤的数据
	Where(filter func(name, value string) bool) map[string]string
	// Fill 使用环境变量填充结构体
	Fill(structure any) error
}

type Environ interface {
	Signer
	// Load 加载定义环境变量的文件
	Load(filenames ...string) error
	// Signed 返回复合一个规则的签名查询器
	Signed(prefix, category string) Signer
	// Clean 清理缓存的所有数据
	Clean()
}

var (
	// 全局缓存的环境变量
	env = New().(*environ)
	// 环境变量文件 `.env` 所处的目录
	// 一般位于程序的工作目录
	root string
)

// Default 返回默认操作对象
func Default() Environ {
	return env
}

// Init 加载运行目录下的 .env 文件
func Init(root ...string) error {
	var dir string
	if len(root) > 0 {
		dir = root[0]
	}
	if dir == "" {
		dir = "."
	}
	return InitWithDir(dir)
}

// InitWithDir 加载指定录下的 .env 文件
func InitWithDir(dir string) (err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			root = ""
			env.Clean()
		} else {
			root = dir
		}
	}()

	// 重置缓存的环境变量
	root = ""
	env.Clean()

	// 加载系统的环境变量
	result := make(map[string]string)
	for _, value := range os.Environ() {
		parts := strings.SplitN(value, "=", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		result[key] = val
	}
	env.Save(result)

	// 加载 .env 和 .env.local 文件
	err = loadEnv(dir, "")
	if err != nil {
		return err
	}

	// 加载与运行环境相关的环境变量
	appEnv := String("APP_ENV", "prod")
	if len(appEnv) > 0 {
		// 加载 .env.{APP_ENV} 和 .env.{APP_ENV}.local 文件
		err = loadEnv(dir, "."+strings.ToLower(appEnv))
		if err != nil {
			return err
		}
	}

	return
}

func loadEnv(dir, env string) error {
	filename := filepath.Join(dir, ".env"+env)
	if err := Load(filename); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := Load(filename + ".local"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Load 加载指定的环境变量文件
func Load(filenames ...string) error {
	return env.Load(filenames...)
}

func Signed(prefix, category string) Signer {
	return env.Signed(prefix, category)
}

// Path 基于初始化目录获取目录
func Path(path ...string) string {
	switch len(path) {
	case 0:
		return root
	case 1:
		return filepath.Join(root, path[0])
	default:
		return filepath.Join(root, filepath.Join(path...))
	}
}

func Is(env ...string) bool {
	if len(env) == 0 {
		return false
	}
	if val, exists := Lookup("APP_ENV"); exists {
		for _, s := range env {
			if val == s {
				return true
			}
		}
	}
	return false
}

// IsEnv 判断应用环境是否与给出的一致
//
// Deprecated: 使用 env.Is 方法代理
func IsEnv(env string) bool {
	return String("APP_ENV") == env
}

// Lookup 查看配置
func Lookup(name string) (string, bool) {
	return env.Lookup(name)
}

// Exists 配置是否存在
func Exists(name string) bool {
	return env.Exists(name)
}

// String 取字符串值
func String(name string, value ...string) string {
	return env.String(name, value...)
}

// Bytes 取二进制值
func Bytes(name string, value ...[]byte) []byte {
	return env.Bytes(name, value...)
}

// Int 取整型值
func Int(name string, value ...int) int {
	return env.Int(name, value...)
}

func Float(name string, value ...float64) float64 {
	return env.Float(name, value...)
}

func Duration(name string, value ...time.Duration) time.Duration {
	return env.Duration(name, value...)
}

func Bool(name string, value ...bool) bool {
	return env.Bool(name, value...)
}

// List 将值按 `,` 分割并返回
func List(name string, fallback ...[]string) []string {
	return env.List(name, fallback...)
}

func Map(prefix string) map[string]string {
	return env.Map(prefix)
}

func Where(filter func(name string, value string) bool) map[string]string {
	return env.Where(filter)
}

// Fill 将环境变量填充到指定结构体
func Fill(structure any) error {
	return env.Fill(structure)
}

// All 返回所有值
func All() map[string]string {
	return env.Where(func(name, value string) bool {
		return true
	})
}
