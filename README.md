# CPA Apply Patch Plugin (`apply_patch.dll`)

<p align="center">
  <a href="README.md">简体中文</a> •
  <a href="README_EN.md">English</a>
</p>

> **专为接入 Codex 桌面端的第三方大模型（Google Gemini、Anthropic Claude、DeepSeek 等）打造的官方标准 C ABI 原生补丁工具调用与 OAuth 中继插件。**
>
> 无需任何外部 Python 脚本，单个 DLL 即插即用。**全面支持第三方模型原生调用 `apply_patch` 工具**进行精准代码修改与版本比对，完美呈现官方文件修改对比卡片（TurnDiff），并无缝保留 ChatGPT Pro 账号登录态与 5 小时限额额度条！

---

## 💡 为什么需要本插件？（解决的痛点）

在将外部大模型（如 Gemini、Claude、DeepSeek）通过 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 接入 Codex 时，开发者通常面临三大难题：

1. **无法调用原生 `apply_patch` 工具，补丁体验降级**：
   Codex 官方客户端针对 OpenAI 自身模型设计了一套私有的 `custom_tool_call`（Freeform Patch）协议。当接入第三方模型时，第三方模型无法正确识别和发起该私有工具调用，只能退化为输出普通 Markdown 文本块或改用 Shell 命令（如 `cat <<EOF`）全量覆写文件，不仅丢失了精准差异更新能力，也无法唤起带有文件名、代码行数增减徽章（`+1, -1`）和【打开/对比】按钮的原生文件修改卡片。
2. **第三方代理拦截，导致官方 Pro 额度条与头像消失**：
   若直接使用第三方代理 API Key，Codex 会被强制设置为 API Key 模式，状态栏将无法显示官方 ChatGPT Pro 5 小时使用限额（Usage Limits）与头像。而如果开启 `auth_mode: "chatgpt"`，Codex 向本地代理发送的 ChatGPT OAuth JWT 令牌又会被代理视为无效凭证而返回 `401 Unauthorized`。
3. **传统方案依赖外部脚本或修改内核，维护成本极高**：
   过去往往需要常驻额外的 Python 胶水脚本，或者直接反编译/魔改代理内核二进制。一旦内核或管理客户端自动更新，改动立刻被覆盖失效。

**本插件彻底解决了以上所有痛点！**

---

## ✨ 核心功能

### 1. 🛠️ 原生 `apply_patch` 工具调用闭环（Native Tool Calling & Execution）
- **工具声明注入与适配**：在请求入站时，自动向外部模型注入符合 Codex V4A 补丁规范的标准 `apply_patch` 工具定义，使 Gemini、Claude、DeepSeek 等第三方模型能够精准识别并主动发起针对文件的增量修改调用；
- **V4A 提示词智能增强**：自动注入 Unified Diff 格式指引，引导模型使用精准的行修改语法，告别全文件重写带来的性能损耗与上下文浪费；
- **多轮会话参数与输出闭环**：模型调用 `apply_patch` 后，插件自动将客户端执行结果以 `custom_tool_call_output` 平滑映射回模型，确保多轮上下文持续感知修改结果与冲突。

### 2. 🎨 官方原生文件变化卡片（TurnDiff Native Card）
- **双向协议转译**：在请求端将 Codex 的 `custom: apply_patch` 声明透明转换为通用模型支持的标准 Function Calling 格式，并注入 V4A Unified Diff 格式指引；
- **5 帧事件流重组**：在响应端实时截获上游模型的函数调用，精准重构为 Codex 渲染引擎严格要求的 5 帧原生 SSE 事件序列：
  - `response.output_item.added` (`type: "custom_tool_call"`)
  - `response.custom_tool_call_input.delta` (代码块增量)
  - `response.custom_tool_call_input.done` (补丁完成)
  - `response.output_item.done` (`status: "completed"`)
  - `response.completed` (整体响应完结)
- **完美视觉呈现**：直接在 Codex UI 中弹出官方原生文件对比卡片，支持直观的绿红增减行数标识与一键回跳对比。

### 3. 🔐 ChatGPT OAuth 登录中继（OAuth Relay Authentication）
- 基于 CLIProxyAPI 官方 `FrontendAuthProvider` 扩展能力；
- 自动识别并放行 Codex 发送的官方 ChatGPT OAuth JWT 令牌（`Bearer eyJ...`），同时兼顾 `sk-geminipro` 等本地 API Key；
- **效果**：Codex 状态栏可**同时保持官方登录态，实时查看 Pro 订阅限额、头像与用量**，而核心推理请求直接走本地代理调度到外部模型。

### 4. 🛡️ 全链路 Panic 隔离与稳定性保障（Zero-Crash Guard）
- 导出给宿主进程的所有 C ABI 入口（`cliproxy_plugin_init`、`cliproxyPluginCall`、`cliproxyPluginFree`）均配置全量异常捕获与恢复（`defer recover()`）；
- 零正则、全原生切片处理，彻底杜绝插件运行时 panic 引发 `cli-proxy-api.exe` 内核崩溃闪退。

### 5. 🚀 零外部依赖，极简即插即用
- 纯 Go 编译为单个动态链接库（`apply_patch.dll`），完全不需要系统安装 Python，不需要运行任何 `.bat` 或 `.ps1` 辅助程序；
- 随内核启动而自动加载，内核升级覆盖亦不影响插件独立存续。

---

## 📊 方案对比

| 特性对比 | 裸代理直连 | 传统 MCP 模式 | 外部 Python 脚本模式 | **本插件 (`apply_patch.dll`)** |
| :--- | :---: | :---: | :---: | :---: |
| **apply_patch 工具调用** | ❌ 无法识别/报错 | ⚠️ 降级为外部 MCP | ⚠️ 易被覆盖 | **✅ 原生支持与多轮闭环** |
| **文件变化卡片** | ❌ 纯文本/Markdown | ⚠️ 仅标准工具弹窗 | ⚠️ 需手动刷配置 | **✅ 100% 官方原生 TurnDiff 卡片** |
| **行数变化徽章 (`+1/-1`)** | ❌ 无 | ❌ 无 | ⚠️ 取决于脚本 | **✅ 完美支持** |
| **Pro 额度条/头像保留** | ❌ 丢失 | ❌ 丢失 | ❌ 冲突报错 | **✅ 完美保留（OAuth Relay）** |
| **运行依赖** | 无 | Node.js / MCP | Python 环境 + 批处理 | **✅ 零依赖（单个 DLL）** |
| **抗内核更新** | ⚠️ 无卡片 | ⚠️ 易抢占冲突 | ❌ 更新即覆盖失效 | **✅ 独立动态库，永久生效** |
| **稳定性与性能** | - | 进程间通信开销 | 频繁读写磁盘冲突 | **✅ 微秒级流式转译，Panic 隔离** |

---

## 🏗️ 技术架构

```text
 ┌────────────────────────────────────────────────────────┐
 │                  Codex Desktop 客户端                   │
 │  - 显示 Pro 额度条 & 官方头像 (ChatGPT OAuth 登录态)     │
 │  - 请求: 发送 custom_tool_call (apply_patch)           │
 │  - 渲染: 原生 TurnDiff 文件变化卡片 (+1/-1, 打开/对比)  │
 └───────────────────────────┬────────────────────────────┘
                             │ HTTP (Responses API / SSE)
                             ▼
 ┌────────────────────────────────────────────────────────┐
 │               CLIProxyAPI (宿主代理内核)               │
 │                                                        │
 │   ┌────────────────────────────────────────────────┐   │
 │   │       CPA Apply Patch Plugin (动态链接库)       │   │
 │   │                                                │   │
 │   │  1. FrontendAuthProvider (OAuth 鉴权放行)      │   │
 │   │     -> 放行 ChatGPT JWT，确保客户端不报 401    │   │
 │   │                                                │   │
 │   │  2. RequestInterceptor / RequestNormalizer     │   │
 │   │     -> 将 custom: apply_patch 转为标准函数定义 │   │
 │   │     -> 注入 V4A Unified Diff 提示词引导        │   │
 │   │     -> 兼容转译多轮历史 custom_tool_call       │   │
 │   │                                                │   │
 │   │  3. StreamChunkInterceptor (出站流重打包)      │   │
 │   │     -> 截获上游 functionCall                   │   │
 │   │     -> 动态重打包为 5 帧原生 custom_tool_call   │   │
 │   └────────────────────────────────────────────────┘   │
 └───────────────────────────┬────────────────────────────┘
                             │ 标准 Function Calling
                             ▼
 ┌────────────────────────────────────────────────────────┐
 │       上游大模型 (Gemini 3.8 / Claude / DeepSeek 等)   │
 └────────────────────────────────────────────────────────┘
```

---

## 📦 快速安装与使用

### 步骤 1：获取插件
从本仓库的 [Releases](../../releases) 页面下载预编译好的 `apply_patch.dll`。

### 步骤 2：放入插件目录
将 `apply_patch.dll` 复制到你的 CLIProxyAPI / EasyCLIProxyAPI 插件文件夹：

```text
# 推荐放置路径（64 位 Windows 标准目录）
<CLIProxyAPI 根目录>\plugins\windows\amd64\apply_patch.dll

# 或者简易目录
<CLIProxyAPI 根目录>\plugins\apply_patch.dll
```

### 步骤 3：确认代理配置 (`config.yaml`)
确保 `config.yaml` 启用了插件加载能力（EasyCLIProxyAPI 默认已开启）：

```yaml
plugins:
  enabled: true
  dir: "plugins"
```

### 步骤 4：重启内核与 Codex
1. 重启 CLIProxyAPI（或 EasyCLIProxyAPI）；
2. 打开 Codex 桌面端，直接开始对话与代码修改！

---

## 🔨 源码编译指南

如果你希望自行编译动态库，请确保本地已安装 Go（1.21 或更高版本，开启 CGO）及 GCC 编译工具链（Windows 推荐 [w64devkit](https://github.com/skeeto/w64devkit) 或 MinGW-w64）。

### Windows 环境一键编译：
```cmd
git clone https://github.com/<your-username>/cpa-apply-patch-plugin.git
cd cpa-apply-patch-plugin
build.bat
```

### Linux / 跨平台交叉编译：
```bash
git clone https://github.com/<your-username>/cpa-apply-patch-plugin.git
cd cpa-apply-patch-plugin
chmod +x build.sh
./build.sh
```

编译成功后将在根目录下生成 `apply_patch.dll`。

---

## ❓ 常见问题 (FAQ)

#### Q1: 为什么使用第三方模型时看不到 Codex 的 Pro 额度条了？
**A**: 这是因为之前你的 `~/.codex/auth.json` 被设为了 `auth_mode: "apikey"`。只需保持 `auth_mode: "chatgpt"` 并保留你的官方 `tokens`。本插件内置了 `FrontendAuthProvider`，在接收到官方 OAuth Token 时会自动放行，兼顾官方额度显示与第三方模型转发。

#### Q2: 为什么我的代码补丁变成了 Markdown 代码块，没有出现卡片？
**A**: 常见原因有两点：
1. `~/.codex/config.toml` 中残存了 `model_provider = "cpa-gui"` 字段（非空值会强制将 Codex 降级为普通第三方通道，剥离原生补丁支持）；将其注释为 `# model_provider = "cpa-gui"` 并使用 `openai_base_url` 即可。
2. 之前安装了同名的 MCP 服务器抢占了调用优先级；可通过 `codex mcp remove apply_patch` 清理冲突。

#### Q3: 插件会导致 CLIProxyAPI 启动闪退吗？
**A**: 绝对不会。本插件严格遵循 C ABI 规范，全流程无耗时阻塞，并在所有导出接口中增加了全方位的 `defer recover()` 保护层，即使遭遇极端异常也不会波及代理内核。

---

## 📄 许可证 (License)

本项目基于 [MIT License](LICENSE) 许可证开源，可自由商用、修改与分发。

---

## 🤝 鸣谢与参考

- [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) - 强大的全模型转译中继网关
- [Codex CLI](https://github.com/openai/codex) - OpenAI 官方开源代码助手
