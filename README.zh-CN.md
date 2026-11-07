# mygo-plugin

[English](README.md) · 简体中文

[![Listed on Awesome MyGo](https://awesomemygo.dev/badge.svg)](https://awesomemygo.dev/plugins)

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
- 从文件夹、ZIP 或 GitHub 安装；支持 id、版本和校验和；网络与进程权限需要显式同意
- 公开的 `mygo-music-plugin` topic 注册表，每周重建；`index` 会编译模块、记录导出，不执行
- 从注册表生成的静态商店页
- 宿主全局的 TypeScript 声明，以及给插件作者的 `plugintest` 包
- 通用宿主渲染页面契约：`contributes.pages` 与 `page({ id })`，不只是 MyGo Music 音源
- 命令（`contributes.commands`、`RunCommand`）、带类型的设置、能力协商（`contract`、`host.capability`）
- Page v1：Markdown 详情正文和条目动作，连同 sections/items
- `mygo-plugin` CLI：`check`、`pack`、`new`、`list`、`index`、`install`、`site`、`dev`
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
| `host.t(text)` | 按用户语言从 `locales/<language>.json` 取译文，没有就用原文 |
| `host.setting(key)` | 用户给这个插件的设置值 |
| `host.capability(name)` | 宿主是否提供某项能力，如 `network`、`clipboard` |
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
- 应用可以用 `Options.Allowed` 再收窄，runtime 启动时逐个 host 询问
- `process` 是声明，不是自动授权

## 注册表

`registry.json` 收录 `mygo-music-plugin` topic 下的插件，按插件 id 去重：目前 29 个仓库得到 23 个插件。每条带权限、GitHub 给出时的 release zip 校验和，以及健康记录：`index` 会编译模块但不执行，留下导出或编译错误。

```sh
GITHUB_TOKEN=... mygo-plugin index -o registry.json  # token 也用来读取 release 校验和
mygo-plugin install netease-haruhikage /path/to/plugins --allow-network
```

两个仓库用同一个 id 时，版本新的优先，版本相同则仓库名排序靠前的优先。安装插件是一次信任决定：命令会打印网络和进程权限，没有 `--allow-network` 和 `--allow-process` 不会继续。注册表由 GitHub Actions 每周刷新。

每条还会带上 GitHub 的信任信号：stars、最近提交、license、是否归档。只抓公开仓库。

`mygo-plugin site` 把同一份注册表写成[静态页面](https://yxxbc.github.io/mygo-plugin/)，带权限、健康、信任信号和安装命令，不需要服务器，也不需要 JavaScript。

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

# 从注册表安装，带校验和与显式同意
mygo-plugin install netease-haruhikage /path/to/plugins --allow-network --yes
# 指定版本，按仓库安装
mygo-plugin install -version 0.2.1 -sha256 <hex> owner/repo /path/to/plugins --yes
```

`install` 按 id 读 `registry.json`，取最新 release 或指定版本，已知校验和时校验，原子安装。需要网络或起进程的插件必须加 `--allow-network` / `--allow-process`：这是用户的同意，不是插件的声明。没有终端时加 `--yes`。

## 给插件作者

- [`spec/host.d.ts`](spec/host.d.ts) 声明 `host` 全局；`plugin.HostTypeScript` 返回同一段文本，版本是 `HostTypeScriptVersion`
- `plugintest.New(t, dir, plugin.Options{...})` 用临时数据目录、10 秒调用超时和 `t.Logf` 加载插件写测试
- `mygo-plugin dev .` 文件变化时重新检查 manifest 并编译模块，打印导出或错误
- [`spec/plugin.schema.json`](spec/plugin.schema.json) 在编辑器里检查 `plugin.json`

## 原生页面（实验）

插件可以返回宿主渲染的页面：标题、分区和条目，没有视图树、HTML 或 CSS。`plugin.Page(id)` 调插件的 `page` 导出；MyGo Music 插件没有它时回退到 `browse`，再解码成 `Page`、`Section`、`Item`。

不是音源的插件声明页面挂在哪：

```json
{
  "contributes": {
    "pages": [{"id": "home", "title": "Home", "icon": "home"}]
  }
}
```

并导出 `page({ id })`。`plugin.Manifest.Pages()` 返回这些入口；没有时返回 MyGo Music 的 `navigation`。宿主用原生组件画两者；MyGo Music 更丰富的 sections（歌曲、歌单、播客）由宿主适配。

页面还可以带 `markdown` 详情正文，条目可以列 `actions`（`kind: "command"` 或 `"url"`），由宿主决定怎么画。命令在 `contributes.commands` 声明，通过 `plugin.RunCommand(id, args)` 执行；设置在 `settings` 里声明 `type`、`default`、`options`，宿主渲染表单。[`spec/page.schema.json`](spec/page.schema.json) 描述 JSON，[`docs/contract-v2.md`](docs/contract-v2.md) 描述契约，[`docs/native-ui.md`](docs/native-ui.md) 说明宿主负责什么。这个形状仍是实验性的。

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

完整调研和规划见 [docs/roadmap.md](docs/roadmap.md)。下一个里程碑是 **Contract v2**：通用页面、命令、设置和能力协商，MyGo Music 只是其中一个适配器，不再是模型。之后：持久授权、签名、进程权限、生成宿主类型，以及两个真实宿主应用。

## 参考宿主

[`examples/host`](examples/host) 是一个用本模块跑插件的小型 MyGo 原生应用：通用页面来自 `contributes.pages`，MyGo Music 的 `navigation` 和 `browse` 通过 `adapters/music` 适配；条目动作、命令栏、Markdown 详情、网络同意卡都在，播放由 Go 下载音频流。它是“应用如何托管插件”的样板，不是成品。

## 版本

[`CHANGELOG.md`](CHANGELOG.md) 记录每个版本。1.0 之前，minor 版本加功能、可能改 API，patch 版本修 bug；按里程碑发版，最多每周一次，不是每次改动都发；规则见 [`RELEASING.md`](RELEASING.md)。manifest 和宿主 API 在 1.0 冻结。

## 来源与许可

MIT。`plugin.json` 的字段形状与 MyGo Music 的插件兼容；本仓库是独立实现，不包含 MyGo Music 的代码。MyGo 是 [egoist/mygo](https://github.com/egoist/mygo)（MIT）。
