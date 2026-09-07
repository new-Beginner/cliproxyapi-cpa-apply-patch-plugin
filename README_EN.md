# CPA Apply Patch Plugin (`apply_patch.dll`)

<p align="center">
  <a href="README.md">简体中文</a> •
  <a href="README_EN.md">English</a>
</p>

> **A standardized C ABI dynamic plugin for [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) enabling native `apply_patch` tool calling and OAuth relay for third-party LLMs (Google Gemini, Anthropic Claude, DeepSeek, etc.) in Codex Desktop.**
>
> Zero external Python scripts, single DLL plug-and-play. **Resolves the fundamental barrier preventing third-party models from calling Codex's native `apply_patch` Freeform tool**, enables seamless tool declaration injection and multi-turn execution feedback, activates official TurnDiff cards, and preserves your official ChatGPT Pro session, avatar, and 5-hour quota bar!

---

## 💡 Why This Plugin? (Root Problem Analysis)

When connecting third-party LLMs (e.g. Google Gemini, Anthropic Claude, DeepSeek) to Codex Desktop via proxy gateways, the primary obstacle is an **architectural tool protocol mismatch**:

1. **Root Problem: Third-party LLMs cannot recognize or invoke Codex's proprietary `apply_patch` Freeform Tool**
   - **Proprietary Freeform Tool Protocol**: Codex Desktop implements a private `custom_tool_call` (Freeform Patch) mechanism specifically for OpenAI models. Instead of taking standard structured JSON parameters, it streams raw V4A Unified Diff patch text directly in an unstructured payload;
   - **Ecosystem Incompatibility**: Models like Gemini, Claude, and DeepSeek strictly adhere to standard JSON Schema Function Calling specifications and **cannot natively parse or trigger Codex's proprietary Freeform tools**;
   - **The Chain of Failures Caused by Inability to Invoke `apply_patch`**:
     - 💥 **Destructive & Uncontrolled File Mutation**: Unable to call `apply_patch` for surgical diffs, the model is forced to dump massive Markdown code blocks in chat or execute crude terminal commands (such as `cat <<EOF > file`) to completely overwrite files. This wastes tokens, inflates latency, and frequently introduces regressions or accidental deletions;
     - 🎨 **Complete UI & Visual Card Breakdown**: Because the client engine receives no valid `custom_tool_call` event stream, Codex Desktop **fails to activate its official native TurnDiff cards** (the interactive diff view with file paths, line change counters `+1, -1`, and graphical [Open/Compare] buttons), reducing the user interface to plain chat text;
     - 🔄 **Broken Multi-Turn Review Feedback**: The model receives no structured `custom_tool_call_output` execution results from the client, causing it to lose context and hallucinate during multi-turn refactoring tasks.
2. **Third-Party Key Requirement Breaks Official Pro Quota & Avatar**:
   Using a static gateway API key forces Codex into `auth_mode: "apikey"`, which prevents the Codex client from fetching user profile and ChatGPT Pro 5-hour usage limit bars from `chatgpt.com/backend-api`. Conversely, if `auth_mode: "chatgpt"` is preserved, the proxy rejects the incoming ChatGPT OAuth JWT token with `401 Unauthorized`.
3. **Fragile External Scripts & Core Binary Tampering**:
   Previous workarounds required running background Python helper scripts or directly patching proxy core binaries. Any proxy core update or GUI reset wipes out these changes immediately.

**This plugin solves all of these challenges in a single, robust dynamic library.**

---

## ✨ Key Features

### 1. 🛠️ Native `apply_patch` Freeform Dynamic Bridging & Tool Calling Loop
- **Freeform ↔ Function Calling Dual-Bridge**: On inbound requests, automatically bridges Codex's `custom: apply_patch` Freeform declaration into standard JSON Schema function tool specifications that external models understand, enabling surgical, line-by-line diff edits;
- **V4A Guidance Enhancement**: Automatically injects Unified Diff prompt guidelines, ensuring models output correct chunk markers without resorting to inefficient full-file rewrites;
- **Multi-Turn Argument & Output Bridging**: Seamlessly maps `custom_tool_call_output` responses back into the model's history, maintaining full context awareness of patch results across multiple turns.

### 2. 🎨 Official Native TurnDiff Cards
- **Protocol-Level Cloaking**: Transparently restores standard model function calls back into Codex's private Freeform patch protocol;
- **5-Frame Event Stream Reconstruction**: Intercepts upstream model function calls and precisely repacks them into the exact 5-frame SSE event stream expected by Codex's native diff rendering engine:
  - `response.output_item.added` (`type: "custom_tool_call"`)
  - `response.custom_tool_call_input.delta` (incremental patch chunk)
  - `response.custom_tool_call_input.done` (patch complete)
  - `response.output_item.done` (`status: "completed"`)
  - `response.completed` (turn finished)
- **Flawless Visual Experience**: Directly launches the official TurnDiff card in Codex UI with line counter badges and one-click code inspection.

### 3. 🔐 ChatGPT OAuth Relay Authentication
- Leverages CLIProxyAPI's `FrontendAuthProvider` plugin capability;
- Automatically authenticates incoming ChatGPT OAuth JWT tokens (`Bearer eyJ...`) sent by Codex, while retaining full compatibility with local keys like `sk-geminipro`;
- **Result**: Codex status bar continues to display **official ChatGPT Pro subscription limits and avatar**, while inference traffic routes locally through external models.

### 4. 🛡️ Zero-Crash C ABI Panic Isolation
- All exported C ABI boundaries (`cliproxy_plugin_init`, `cliproxyPluginCall`, `cliproxyPluginFree`) are protected with full `defer recover()` guards;
- Zero regex, memory-safe pure Go string slice operations prevent any possibility of runtime panics causing the host `cli-proxy-api.exe` process to crash.

### 5. 🚀 Zero-Touch Self-Contained Deployment
- Pure Go binary compiling into a single DLL (`apply_patch.dll`);
- No Python runtime required, no `.bat` or `.ps1` background scripts;
- Independent of proxy core binaries: persists safely across CLIProxyAPI core updates.

---

## 📊 Feature Comparison

| Capability | Raw Gateway | Generic MCP Server | External Python Script | **This Plugin (`apply_patch.dll`)** |
| :--- | :---: | :---: | :---: | :---: |
| **apply_patch Tool Calling** | ❌ Rejected / Error | ⚠️ Generic MCP fallback | ⚠️ Prone to overwrite | **✅ Native Support & Multi-Turn Loop** |
| **Native TurnDiff Card** | ❌ Markdown only | ⚠️ Generic tool popup | ⚠️ Requires manual script | **✅ 100% Native TurnDiff Card** |
| **Line Counter Badges (`+1/-1`)** | ❌ None | ❌ None | ⚠️ Unstable | **✅ Fully Supported** |
| **Pro Quota / Avatar Retention** | ❌ Lost | ❌ Lost | ❌ Auth Conflict | **✅ Fully Preserved (OAuth Relay)** |
| **Runtime Dependencies** | None | Node.js / MCP | Python + Shell Scripts | **✅ Zero (Single DLL)** |
| **Resilience to Core Updates** | ⚠️ No cards | ⚠️ MCP priority conflicts | ❌ Overwritten on update | **✅ Standalone DLL, Permanent** |
| **Performance & Latency** | - | IPC process overhead | Disk I/O race conditions | **✅ Microsecond stream translation** |

---

## 🏗️ Technical Architecture

```text
 ┌────────────────────────────────────────────────────────┐
 │                   Codex Desktop Client                 │
 │  - Displays Pro Quota & User Avatar (ChatGPT OAuth)   │
 │  - Inbound: Emits custom_tool_call (apply_patch)       │
 │  - Rendering: Native TurnDiff card (+1/-1, Open button)│
 └───────────────────────────┬────────────────────────────┘
                             │ HTTP (Responses API / SSE)
                             ▼
 ┌────────────────────────────────────────────────────────┐
 │                  CLIProxyAPI (Host Core)               │
 │                                                        │
 │   ┌────────────────────────────────────────────────┐   │
 │   │       CPA Apply Patch Plugin (Dynamic DLL)     │   │
 │   │                                                │   │
 │   │  1. FrontendAuthProvider (OAuth Relay Auth)    │   │
 │   │     -> Approves ChatGPT JWT, avoids 401        │   │
 │   │                                                │   │
 │   │  2. RequestInterceptor / RequestNormalizer     │   │
 │   │     -> Downgrades custom patch to standard FC  │   │
 │   │     -> Injects V4A Unified Diff prompt rules   │   │
 │   │     -> Translates multi-turn history items     │   │
 │   │                                                │   │
 │   │  3. StreamChunkInterceptor (Stream Repacker)   │   │
 │   │     -> Intercepts upstream functionCall        │   │
 │   │     -> Repacks into 5-frame custom_tool_call   │   │
 │   └────────────────────────────────────────────────┘   │
 └───────────────────────────┬────────────────────────────┘
                             │ Standard Function Calling
                             ▼
 ┌────────────────────────────────────────────────────────┐
 │     Upstream LLMs (Gemini 3.8 / Claude / DeepSeek)     │
 └────────────────────────────────────────────────────────┘
```

---

## 📦 Installation & Quick Start

### Step 1: Download the Plugin
Download the precompiled `apply_patch.dll` from this repository's [Releases](../../releases).

### Step 2: Place in Plugins Directory
Copy `apply_patch.dll` into your CLIProxyAPI / EasyCLIProxyAPI plugins directory:

```text
# Recommended standard path (64-bit Windows)
<CLIProxyAPI Root>\plugins\windows\amd64\apply_patch.dll

# Or root plugin directory
<CLIProxyAPI Root>\plugins\apply_patch.dll
```

### Step 3: Verify Configuration (`config.yaml`)
Ensure plugin loading is enabled in `config.yaml` (enabled by default in EasyCLIProxyAPI):

```yaml
plugins:
  enabled: true
  dir: "plugins"
```

### Step 4: Restart Core & Codex
1. Restart CLIProxyAPI (or EasyCLIProxyAPI);
2. Launch Codex Desktop and enjoy native diff cards with your favorite models!

---

## 🔨 Building from Source

Requirements: Go 1.21 or later with CGO enabled, and a C compiler (e.g. [w64devkit](https://github.com/skeeto/w64devkit) or MinGW-w64 on Windows).

### Windows One-Click Build:
```cmd
git clone https://github.com/<your-username>/cpa-apply-patch-plugin.git
cd cpa-apply-patch-plugin
build.bat
```

### Linux / Cross-Compilation:
```bash
git clone https://github.com/<your-username>/cpa-apply-patch-plugin.git
cd cpa-apply-patch-plugin
chmod +x build.sh
./build.sh
```

The build artifact `apply_patch.dll` will be generated in the project root.

---

## ❓ FAQ

#### Q1: Why can't I see my ChatGPT Pro quota bar when using external models?
**A**: Make sure `~/.codex/auth.json` is set to `auth_mode: "chatgpt"` with your original `tokens` preserved. This plugin's `FrontendAuthProvider` will transparently accept your ChatGPT OAuth tokens so both the official UI quota and proxy routing work at the same time.

#### Q2: Why are file changes displayed as Markdown blocks instead of native diff cards?
**A**: Check the following:
1. Ensure `model_provider` in `~/.codex/config.toml` is commented out (`# model_provider = "cpa-gui"`), allowing Codex's default engine to send native patch tool declarations to `openai_base_url`.
2. If an older MCP patch server was installed, remove it with `codex mcp remove apply_patch` to prevent tool hijacking.

#### Q3: Does this plugin introduce any stability risks or crash `cli-proxy-api.exe`?
**A**: No. The plugin implements comprehensive panic recovery across all C ABI entry points and uses safe Go string slice manipulation, ensuring zero impact on host stability.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).

---

## 🤝 Acknowledgments

- [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) - The universal proxy gateway for LLMs
- [Codex CLI](https://github.com/openai/codex) - OpenAI's open-source coding assistant
