# env — Go 环境变量管理

![CI](https://github.com/go-slim/env/actions/workflows/ci.yml/badge.svg)

中文 | [English](README.md)

一个小巧、轻量级的环境变量加载和查询工具，具有以下特性：

- 通过 Signer 实现作用域查找：使用 `PREFIX` 和可选的 `CATEGORY`（例如 `CACHE_BOOK_*` 回退到 `CACHE_*`）。
- 简单的类型获取器：`String`、`Bytes`、`Int`、`Bool`、`Duration`、`List`。
- 通过标签填充结构体：使用 `env:"KEY"` 标签调用 `s.Fill(&cfg)`。
- 全局辅助函数和 `.env` 加载链：`.env`、`.env.local`、`.env.<APP_ENV>`、`.env.<APP_ENV>.local`。

模块路径：`go-slim.dev/env`

## 设计理念

本库遵循**"初始化一次，多次读取"**的模式：

1. **初始化阶段**：在应用启动时一次性加载环境变量（通常在 `main()` 中）
2. **运行时阶段**：在多个 goroutine 中安全地读取环境变量

### 线程安全模型

| 操作                                     | 线程安全性    | 使用时机       |
| ---------------------------------------- | ------------- | -------------- |
| `Init()`, `InitWithDir()`, `Load()`      | ❌ 非线程安全 | 启动时调用一次 |
| `Lookup()`, `String()`, `Int()` 等       | ✅ 线程安全   | 可并发使用     |
| `Signed()`, `Fill()`, `Map()`, `Where()` | ✅ 线程安全   | 可并发使用     |

**关键点**：初始化函数会修改全局状态，必须在启动 goroutine 之前调用。

## 安装

```bash
go get go-slim.dev/env
```

Go 版本：`1.22`（根据 `env/go.mod`）。

## 快速开始

```go
package main

import (
    "fmt"
    "log"
    env "go-slim.dev/env"
)

func main() {
    // ✓ 初始化：在启动时调用 Init() 一次（单线程）
    if err := env.Init(); err != nil {
        log.Fatal(err)
    }

    // ✓ 最佳实践：初始化后锁定
    env.Lock()

    // ✓ 运行时：可以安全地从多个 goroutine 读取
    go worker1()
    go worker2()
}

func worker1() {
    // Signed 查找通过前缀和可选类别进行分组。
    // 键匹配规则：首先尝试 PREFIX_CATEGORY_KEY，然后回退到 PREFIX_KEY。
    cache := env.Signed("CACHE", "BOOK")

    // 优先使用类别级别
    driver := cache.String("DRIVER")         // 从 CACHE_BOOK_DRIVER（或 ""）
    // 回退到前缀级别
    database := cache.Int("DATABASE")        // 从 CACHE_BOOK_DATABASE，否则从 CACHE_DATABASE

    fmt.Println(driver, database)
}

func worker2() {
    // 直接读取也是安全的
    port := env.Int("PORT", 8080)
    fmt.Println("端口:", port)
}
```

### ❌ 错误用法

```go
func main() {
    // 错误：不要并发调用 Init()！
    go env.Init()  // 数据竞争！
    go env.Init()  // 数据竞争！

    // 错误：不要在运行时修改环境变量！
    go func() {
        env.Load(".env.runtime")  // 如果其他 goroutine 正在读取会导致数据竞争！
    }()
}
```

## Signed 查找

假设有以下环境变量：

```
CACHE_DRIVER=redis
CACHE_DATABASE=1
CACHE_SCOPE=app:
CACHE_BOOK_DATABASE=10
CACHE_BOOK_SCOPE=app:books:
```

你可以通过类别作用域查询：

```go
cache := env.Signed("CACHE", "BOOK")
_ = cache.String("DRIVER")   // "redis"（从 CACHE_DRIVER 回退）
_ = cache.Int("DATABASE")     // 10（来自 CACHE_BOOK_DATABASE）
_ = cache.String("SCOPE")     // "app:books:"（来自 CACHE_BOOK_SCOPE）
```

内部实现上，解析器首先尝试 `PREFIX_CATEGORY_KEY`；如果缺失或为空，则回退到 `PREFIX_KEY`。

## 全局辅助函数

- `env.Init(root ...string)` 和 `env.InitWithDir(dir string)` 加载：
  - `.env`、`.env.local`、`.env.<APP_ENV>`、`.env.<APP_ENV>.local`
  - 默认的 `APP_ENV` 是 `prod`（也可通过 `env.String("APP_ENV")` 访问）。
- `env.Path(...)` 返回初始化根目录（可选择性地连接路径）。
- `env.Is("dev", "prod", ...)` 检查 `APP_ENV` 是否匹配任何提供的值。
- `env.All()` 返回所有已加载的键值对。
- `env.Load(files...)` 读取额外的 `.env` 文件并合并值。
- `env.Read(r io.Reader)` 从 `io.Reader` 读取并解析环境变量（实例级别方法）。
- `env.Updates(map[string]string)` 将内存中的值更新到全局环境中。
- `env.Read(r io.Reader)` 从 `io.Reader` 读取并解析环境变量（实例级别方法）。

以上所有函数在 `env.Environ` 上都有对应的实例级别方法。

### Read 方法

`Read` 方法允许从任意 `io.Reader` 源（如字符串、文件、网络流等）读取环境变量：

```go
import (
    "strings"
    env "go-slim.dev/env"
)

// 从字符串读取环境变量
envData := `
APP_NAME=MyApp
APP_PORT=8080
DEBUG=true
`
e := env.New()
_ = e.Read(strings.NewReader(envData))

// 使用读取的环境变量
fmt.Println(e.String("APP_NAME")) // "MyApp"
fmt.Println(e.Int("APP_PORT"))     // 8080
fmt.Println(e.Bool("DEBUG"))       // true
```

## 类型获取器

在 `Signer` 和 `Environ` 上都可用：

- `String(key, ...fallback)`
- `Bytes(key, ...fallback)`
- `Int(key, ...fallback)`
- `Bool(key, ...fallback)`
- `Duration(key, ...fallback)` — 解析 Go 持续时间字符串（例如 `"150ms"`、`"2s"`）或将整数作为纳秒。
- `List(key, ...fallback)` — 按逗号分隔并修剪空格。

## 结构体填充

使用 `env:"KEY"` 标签从环境变量填充结构体字段：

```go
type Feature struct {
    Flag bool `env:"FEATURE_FLAG"`
}

type Config struct {
    Host    string  `env:"HOST"`
    Port    int     `env:"PORT"`
    Debug   bool    `env:"DEBUG"`
    Timeout string  `env:"TIMEOUT"` // 作为字符串存储；如需要可解析为 time.Duration
    Rate    float64 `env:"RATE"`
    Feat    Feature             // 支持嵌套结构体
    Opt     *Feature            // 支持指向结构体的指针（如果非 nil）
}

func load() (*Config, error) {
    _ = env.Init() // 或 InitWithDir
    cfg := &Config{Opt: &Feature{}}
    if err := env.Signed("APP", "WEB").Fill(cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

注意事项：

- 填充器利用 `reflect` 和轻量级类型转换辅助函数。
- 如果需要 `time.Duration`，在结构体中保持为 `string` 类型，并通过 `time.ParseDuration` 解析。

## 迭代、Map 和 Where

- `Environ.Map(prefix)` 收集具有给定前缀的键，并返回去除前缀的映射。
- `Environ.Where(func(name, value string) bool)` 过滤所有键值对。
- 底层实现上，`Signer` 还通过 `PREFIX_CATEGORY_` 过滤迭代，并缓冲 `PREFIX_` 值以供回退使用。

## 测试

在模块目录 `env/` 中：

```bash
go test -v ./
# 或启用竞态检测
go test -race -v ./
```

如果从具有 `go.work` 的多模块仓库根目录运行，你可能更愿意显式测试每个模块：

```bash
go test -v ./env
```

测试套件涵盖：

- Signed 查找和回退语义
- 迭代、缓冲和修剪
- 类型获取器和列表解析
- 结构体 `Fill()`，包括嵌套/指针结构体
- 全局辅助函数：`Init`、`Path`、`Is`、`All`、`Load`、`Updates`

## 许可证

MIT
