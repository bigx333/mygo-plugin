# mygo-plugin

[English](README.md) · 简体中文

MyGo 应用的原生插件宿主：纯 Go、进程内、无 WebView。插件是 `plugin.json` 加一个 JavaScript / TypeScript 模块，由 [moejs](https://github.com/MoeForge/moejs) 在每个插件自己的运行时里跑；界面仍由宿主用 MyGo 的原生 UI 绘制。

这不是又一个 WebView 插件容器。插件拿不到 DOM，也画不了界面；它提供数据，宿主决定怎么画。这样插件再重，应用也还是 MyGo 的应用。

`plugin.json` 与 MyGo Music 的插件同形，现有的网易云等插件可以直接加载。

## 为什么单独做

MyGo 自带的 `mygo.Plugin` 是**编译期**的能力模块：Go service 加 npm 包，`mygo.Use` 注册，只有可信页面能调。它适合官方能力（fetch、sqlite、terminal），不适合安装第三方插件。

MyGo Music 有一套完整的第三方插件运行时，但它在 `internal/plugin` 里，和音乐类型绑死，别的应用拿不走。这个仓库把宿主抽出来：manifest、安装、权限、运行时、限制、CLI，全都不依赖音乐。

## 能力

- 每个插件一个 moejs runtime，独立堆、独立事件循环，互不影响
- ES modules、async/await、TypeScript（`main` 为 `.ts` 时）
- `host` API：storage、crypto（MD5 / AES-ECB）、language、setting、openURL、signInPrompt
- `fetch` 按 `permissions.network` 的 host allowlist 放行；没有权限就完全断网
- 调用超时、256 MiB 内存上限、卡死自动重启
- 从文件夹或 ZIP 安装；ZIP 防穿越、限大小；`List` / `Uninstall`
- `mygo-plugin` CLI：`check`、`pack`、`new`、`list`
- manifest JSON Schema：`spec/plugin.schema.json`

## 快速开始

```go
import plugin "github.com/yxxbc/mygo-plugin"

p, err := plugin.Load("plugins/netease-haruhikage", plugin.Options{
    AppVersion: "0.1.26",
    DataDir:    filepath.Join(configDir, "mygo-plugin"),
    Host: plugin.Host{
        Language:     func() string { return "zh-CN" },
        OpenURL:      func(u string) error { return mygo.Shell.OpenExternal(u) },
        SignInPrompt: func(id string, p plugin.SignInPrompt) { showQR(p.QR, p.Text) },
    },
})
if err != nil {
    return err
}
defer p.Close()

var result struct {
    Tracks []struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    } `json:"tracks"`
}
err = p.Call("search", map[string]any{"query": "周杰伦", "kind": "tracks", "offset": 0}, &result)
```

`Options.DataDir` 必须由应用指定，并且按应用区分：同一个插件装进两个应用，不能共享 session 和 storage。

`Call` 的约定由应用定，不在这层：插件导出什么，应用就调什么。MyGo Music 的来源契约（`search` / `browse` / `stream` / `lyrics` / `signIn` …）是其中一个已存在的契约，现有插件直接可用。

## 原生的边界

插件不能操作宿主的 UI 树。宿主从 `plugin.json` 的 `navigation` 生成侧栏，用户点某一项时：

- 有 `page`：调用插件的 `browse({ page })`，拿回 `{ title, sections: [...] }` 之类的数据，用 MyGo 原生组件画
- 有 `builtin`：宿主自己的页面（首页、喜欢、播客…）

播放的**数据面留在 Go**：插件 `stream` 只返回 `{ url, headers, type }`，宿主自己下载、解码、播放，音频字节不经过 JS。歌词同理，插件返回文本，宿主排版。

MyGo 已经区分可信页面和不可信页面：需要 HTML 的插件 UI 可以放进 `PageOptions{Untrusted: true}` 的 WebView，它调用不了 Go 方法，只能触达应用自己的 handler。这个宿主面向不需要 HTML 的插件：原生 UI 由 Go 绘制，第三方代码画不了。这里的插件没有页面、没有 bridge、没有 DOM，只能看到 manifest 声明过的宿主函数，数据由宿主用原生视图渲染。

完整的威胁模型，以及框架级边界需要回答的问题，见 [docs/trust-model.md](docs/trust-model.md)。

这就是性能的关键：moejs 的调用是微秒级，但真正的省是在于不把音频、封面、歌词塞进运行时的 JSON。保持调用粗粒度，别做万能 RPC。

图标不是文件，是宿主图标集里的名字（`calendar`、`star`、`playlist`…）。macOS 上宿主可以用 `plugins/sfsymbols` 画系统 SF Symbols，其他平台用自己的回退集。插件不该带 SVG，否则每个宿主都要处理栅格化和设计一致性。

## Host API

| 名称 | 说明 |
| --- | --- |
| `host.platform` / `host.arch` | 宿主平台 |
| `host.dir` / `host.dataDir` | 插件目录 / 插件数据目录 |
| `host.storage.get/set/remove` | 每插件独立的 JSON KV，落盘到 `storage.json` |
| `host.crypto.md5` / `host.crypto.aesEcbEncrypt` | 十六进制输出，PKCS#7 填充 |
| `host.language()` | BCP 47 语言标签 |
| `host.setting(key)` | 用户给这个插件的设置值 |
| `host.openURL(url)` | 只允许 http/https |
| `host.signInPrompt({ qr, text })` | 登录时给宿主显示 |
| `host.spawn` | 本版本不启动进程，调用会报错 |

`permissions.process` 会被解析，但当前宿主不执行；需要起进程的宿主自己实现，并保持 deny-by-default。

## 权限

```json
{
  "permissions": {
    "network": ["music.163.com", "interfacepc.music.163.com"],
    "process": ["bin/helper-*"]
  }
}
```

- `network` 为空：`fetch` 不接网络
- `example.com` 精确匹配；`*.example.com` 匹配子域；`*` 全放行
- allowlist 在每次请求（含重定向）时检查
- `process` 是声明，不是自动授权

## 安装与分发

```go
plugin.InstallDir(src, pluginsDir)   // 从文件夹
plugin.InstallZip(zip, pluginsDir)   // 从 ZIP，plugin.json 在根或唯一子目录
plugin.List(pluginsDir)
plugin.Uninstall(pluginsDir, id)     // 保留数据，重装后 session 还在
```

```sh
mygo-plugin check -app-version 0.1.26 ./my-plugin
mygo-plugin pack -o my-plugin.zip ./my-plugin
mygo-plugin new my-plugin
mygo-plugin list ~/Library/Application\ Support/MyApp/plugins
```

签名、registry 索引、从 GitHub release 直接安装还没做；见路线图。别在 v0.1 就自建审核后台。

## 限制

- 内存：默认 256 MiB，超过会被中断，下次调用重启
- 时间：默认 30s 一次调用；卡死在 JS 里会被终止
- 一个插件一个 runtime；调用串行进入该 runtime
- 插件 `import` 不可用（单文件模块）；需要多文件时先打包

## 开发

```sh
go test ./...
go vet ./...

# 用真实插件验证（不会进 CI）
MYGO_PLUGIN_REAL=../mygo-music-netease MYGO_PLUGIN_NETWORK=1 go test -run TestReal -v
```

## 路线图

- 从 GitHub release / topic 安装，deeplink
- Ed25519 签名与安装校验
- 进程权限的真实实现（allowlist + 授权 UI）
- 静态 registry index（GitHub 即源，CI 生成 JSON）
- 从宿主 Go 契约生成 `host.d.ts`，加 mock host 测试包
- 标准图标名表与跨平台回退

## 来源与许可

MIT。`plugin.json` 的字段形状与 MyGo Music 的插件兼容；本仓库是独立实现，不包含 MyGo Music 的代码。MyGo 是 [egoist/mygo](https://github.com/egoist/mygo)（MIT）。
