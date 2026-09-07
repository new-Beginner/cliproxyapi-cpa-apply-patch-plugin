# CPA Apply Patch Plugin (`apply_patch.dll`)

<p align="center">
  <a href="README.md">简体中文</a> •
  <a href="README_EN.md">English</a>
</p>

> **专为接入 Codex 桌面端的第三方大模型（Google Gemini、Anthropic Claude、DeepSeek 等）打造的官方标准 C ABI 原生补丁工具调用与 OAuth 中继插件。**
>
> 无需任何外部 Python 脚本，单个 DLL 即插即用。**解决第三方模型无法调用 Codex 原生 `apply_patch` Freeform 自由格式工具的根本痛点**，打通工具声明注入、参数转译与响应事件闭环，完美唤起官方文件修改对比卡片（TurnDiff），并无缝保留 ChatGPT Pro 账号登录态与 5 小时限额额度条！

---

## 💡 为什么需要本插件？（根本痛点解析）

在将第三方大模型（如 Google Gemini、Anthropic Claude、DeepSeek 等）通过代理网关接入 Codex 桌面端时，核心矛盾在于 **工具协议体系的不兼容**：

1. **根本痛点：第三方大模型无法识别并调用 Codex 原生的 `apply_patch` Freeform 工具**
   - **私有 Freeform 协议**：Codex 官方客户端针对自身模型设计了一套私有的 `custom_tool_call`（即 Freeform Patch 工具机制）。该工具的入参与常规结构化 JSON 参数不同，而是直接以流式纯文本传输 V4A Unified Diff 差异数据；
   - **第三方生态阻断**：Gemini、Claude、DeepSeek 等外部模型天然遵循严格的 JSON Schema Function Calling 契约，**无法识别、更无法主动触发这种私有的 Freeform 工具调用**；
   - **由“无法调用”引发的一系列连锁灾难**：
     - 💥 **文件修改行为粗暴失控**：由于无法调用 `apply_patch` 进行局部增量修补，模型只能被迫退化为在回复中输出冗长的 Markdown 代码块，或者借助终端 Shell 命令（如 `cat <<EOF > file`）进行全量覆写。这种方式极易破坏既有代码、丢失未修改逻辑，并严重浪费上下文与 token；
     - 🎨 **UI 视觉与对比交互完全破裂**：由于底层未能正常触发真实的 `custom_tool_call` 事件流，Codex 桌面端**无法激活官方原生的 TurnDiff 文件变化卡片**（即带有文件路径标签、代码增减 `+1, -1` 徽章和图形化【打开/对比】按钮的高级对比界面），只能沦落为普通的聊天代码框；
     - 🔄 **多轮补丁审查与反馈断流**：模型无法接收到系统针对 patch 执行状态的标准化结构化反馈（`custom_tool_call_output`），在多轮修改与复杂重构中极易陷入幻觉和状态丢失。
2. **第三方代理拦截，导致官方 Pro 额度条与头像消失**：
   若直接使用第三方代理 API Key，Codex 会被强制设置为 API Key 模式，状态栏将无法显示官方 ChatGPT Pro 5 小时使用限额（Usage Limits）与头像。而如果开启 `auth_mode: "chatgpt"`，Codex 向本地代理发送的 ChatGPT OAuth JWT 令牌又会被代理视为无效凭证而返回 `401 Unauthorized`。
3. **传统方案依赖外部脚本或修改内核，维护成本极高**：
   过去往往需要常驻额外的 Python 胶水脚本，或者直接反编译/魔改代理内核二进制。一旦内核或管理客户端自动更新，改动立刻被覆盖失效。

**本插件彻底解决了以上所有痛点！**

---

## ✨ 核心功能

### 1. 🛠️ 原生 `apply_patch` Freeform 工具动态桥接与调用闭环
- **Freeform ↔ Function Calling 双向桥接**：在请求入站时，自动将 Codex 的 `custom: apply_patch` Freeform 声明智能包装为外部模型认识的标准 JSON Schema 函数工具定义，使 Gemini、Claude、DeepSeek 能精准识别并主动发起行级别的精准差异修改；
- **V4A 提示词智能增强**：自动注入 Unified Diff 格式指引，引导模型使用精准的行修改语法，告别全文件重写带来的性能损耗与上下文浪费；
- **多轮会话参数与输出闭环**：模型调用 `apply_patch` 后，插件自动将客户端执行结果以 `custom_tool_call_output` 平滑映射回模型，确保多轮上下文持续感知修改结果与冲突。

### 2. 🎨 官方原生文件变化卡片（TurnDiff Native Card）
- **协议层完美伪装**：将上游模型的标准函数调用，实时转译还原为 Codex 渲染引擎所要求的私有 Freeform 补丁调用；
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

## 🌐 支持的协议与架构规范

本插件采用**多层双向协议转译架构**，针对各端协议提供全方位原生支持与兼容：

### 1. 客户端下行协议（面向 Codex 桌面端 / CLI）
| 协议层级 | 支持的具体协议与规范 | 作用与机制 |
| :--- | :--- | :--- |
| **API 接口协议** | **OpenAI Responses API** (`POST /v1/responses`) | Codex 与后端交互的标准 API 传输通道 |
| **数据传输协议** | **Server-Sent Events (SSE)** 流式传输 (`text/event-stream`) | 逐帧动态截获、重构与分发 SSE 事件流 |
| **工具下行协议** | **Codex 专有 Freeform Patch 协议** (`type: "custom"`) | Codex 专为自身模型设计的高级自由补丁工具格式 |
| **事件流时序协议** | **Codex 原生 5 帧补丁生命周期事件序列** | 精确模拟官方模型序列以激活 TurnDiff 卡片：<br>1. `response.output_item.added` (`type: "custom_tool_call"`)<br>2. `response.custom_tool_call_input.delta` (差异流式增量)<br>3. `response.custom_tool_call_input.done` (补丁数据封包)<br>4. `response.output_item.done` (`status: "completed"`)<br>5. `response.completed` (会话输出项重构) |
| **非流式响应协议** | **JSON Response Output** (`output[].type: "custom_tool_call"`) | 非流式请求下的补丁对象完整重构 |
| **上下文回传协议** | `custom_tool_call` 与 `custom_tool_call_output` | 多轮历史对话中的补丁调用与执行结果平滑兼容 |

### 2. 工具调用与补丁规范协议（Freeform ↔ Function Calling 桥接）
| 补丁与工具规范 | 支持格式 | 详细机制与特性 |
| :--- | :--- | :--- |
| **补丁文本规范** | **Codex V4A Unified Diff 格式** | 严格遵循以 `*** Begin Patch` 开头、`*** End Patch` 结尾，支持 `*** Add File:`、`*** Update File:`、`*** Delete File:` 及单侧 `@@` 差异语法 |
| **标准工具声明** | **JSON Schema Function Calling** | 在请求入站时将 `custom: apply_patch` 包装为标准的 `type: "function"`，入参规范定义为 `{"input": {"type": "string"}}` |
| **参数容错提取协议** | **多结构自动解包 + 转义自动还原** | 模型可能输出不同格式的 JSON，插件自动兼容：<br>• `{"input": "..."}`（官方标准格式）<br>• `{"patch": "..."}` / `{"diff": "..."}`<br>• `{"content": "..."}` / `{"apply_patch": "..."}`<br>• 裸文本/直接附带 `*** Begin Patch` 的字符串<br>• 自动还原 `\n`、`\r`、`\"` 双重转义 |

### 3. 模型端上游协议（面向各类主流大模型）
| 模型厂商 / 通道 | 支持的上游协议类型 | 专属优化处理与特性 |
| :--- | :--- | :--- |
| **Google Gemini** | `gemini`、`gemini-interactions` 原生协议 | 自动注入 `functionDeclarations` 与 `systemInstruction.parts` V4A 补丁格式约束指令 |
| **Google Antigravity** | `antigravity` (Cloud Code 专有内部通道) | 自动适配嵌套的 `request.tools` 与 `request.systemInstruction` 结构 |
| **Anthropic Claude** | Claude Messages API (`tools` / Tool Use 规范) | 在入站前已转为通用标准工具定义，经代理内核无缝转译为 Claude Tool Use |
| **DeepSeek** | OpenAI Chat Completions / Responses 协议 | 转换为标准 `tools: [{"type": "function", ...}]`，支持 DeepSeek 原生函数调用 |
| **OpenAI-Compatible** | 通用 OpenAI 兼容接口 | 标准 Function Calling 通用支持 |

### 4. 客户端鉴权与中继协议（OAuth Relay）
| 认证协议 | 鉴权方式与来源 | 效果与能力 |
| :--- | :--- | :--- |
| **ChatGPT OAuth JWT** | `Authorization: Bearer eyJ...` (RFC 7519 JWT) | **核心特性**：自动放行 Codex 发送的官方 ChatGPT OAuth 令牌，**保持 Codex 桌面端 Pro 5 小时限额额度条、用户头像与官方服务** |
| **标准 API Key** | `Authorization: Bearer sk-...` | 支持 `sk-geminipro` 及用户在 `config.yaml` 中配置的自定义静态密钥 |
| **多字段凭证兼容** | `X-Api-Key`、`X-Goog-Api-Key`、URL Query `?key=...` | 兼容各类客户端与第三方工具的请求头鉴权风格 |

### 5. 宿主插件底层契约（CLIProxyAPI C ABI）
基于 `github.com/router-for-me/CLIProxyAPI/v7`（当前对齐 v7.2.152）官方插件规范构建：
- **C ABI 动态导出**：`cliproxy_plugin_init`、`cliproxyPluginCall`、`cliproxyPluginFree`、`cliproxyPluginShutdown`
- **生命周期拦截器挂载**：
  1. `FrontendAuthProvider` (`frontend_auth.*`)：前端流量认证与 OAuth 中继；
  2. `RequestInterceptor` (`request.intercept_before` / `after`)：入站工具与多轮历史转译；
  3. `RequestNormalizer` (`request.normalize`)：上游模型专有工具声明与提示词增强；
  4. `StreamChunkInterceptor` (`response.intercept_stream_chunk`)：出站流式 5 帧补丁重组；
  5. `ResponseInterceptor` (`response.intercept_after`)：非流式完整响应重构。

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
