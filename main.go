package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	void* call;
	void* free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	RequestInterceptor     bool `json:"request_interceptor"`
	RequestNormalizer      bool `json:"request_normalizer"`
	StreamChunkInterceptor bool `json:"response_stream_interceptor"`
	ResponseInterceptor    bool `json:"response_interceptor"`
	FrontendAuthProvider   bool `json:"frontend_auth_provider"`
}

type identifierResponse struct {
	Identifier string `json:"identifier"`
}

var (
	patchCallMu sync.RWMutex
	patchCalls  = make(map[string]bool)   // call_id -> true
	patchInputs = make(map[string]string) // call_id -> patch text
	fcIDs       = make(map[string]string) // call_id -> item_id (fc_id)
)

func setPatchCall(callID, fcID string) {
	patchCallMu.Lock()
	defer patchCallMu.Unlock()
	patchCalls[callID] = true
	if fcID != "" {
		fcIDs[callID] = fcID
	}
}

func isPatchCall(callID string) bool {
	patchCallMu.RLock()
	defer patchCallMu.RUnlock()
	return patchCalls[callID]
}

func recordPatch(callID, patch string) {
	patchCallMu.Lock()
	defer patchCallMu.Unlock()
	patchInputs[callID] = patch
}

func getPatch(callID string) string {
	patchCallMu.RLock()
	defer patchCallMu.RUnlock()
	return patchInputs[callID]
}

func getFcID(callID string) string {
	patchCallMu.RLock()
	defer patchCallMu.RUnlock()
	if id, ok := fcIDs[callID]; ok && id != "" {
		return id
	}
	return "fc_" + callID
}

func debugLog(format string, a ...any) {
	msg := fmt.Sprintf("[%s] ", time.Now().Format("15:04:05.000")) + fmt.Sprintf(format, a...) + "\n"
	f, err := os.OpenFile("D:\\32057\\Files_of_Desktop\\Academic\\AI\\cpa-apply-patch-plugin\\debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		_, _ = f.WriteString(msg)
	}
}

const applyPatchChatGuidance = `Edit files using the apply_patch tool.
**ALWAYS use this tool to write file content** — new files, single-line edits, and full-file rewrites alike.
**NEVER use shell cat <<EOF > file / echo > file / any > redirect to write actual file content** — doing so bypasses the Codex diff UI and audit trail.
Call this function with a single 'input' string containing a unified patch.
The patch MUST start with '*** Begin Patch' as the literal first line and end with '*** End Patch'.
Headers:
- '*** Add File: <path>'
- '*** Update File: <path>'
- '*** Delete File: <path>'
Within Update hunks, lines start with '-' (removed), '+' (added), or ' ' (context).
Use single-sided '@@ <header>' without trailing '@@'. Relative paths only.`

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(_ *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) (retCode C.int) {
	defer func() {
		if r := recover(); r != nil {
			debugLog("cliproxy_plugin_init PANIC RECOVERED: %v", r)
			retCode = 0
		}
	}()
	if plugin == nil {
		return 1
	}
	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("async ensureCodexEnvironment PANIC RECOVERED: %v", r)
			}
		}()
		ensureCodexEnvironment()
	}()
	return 0
}

func cleanMcpServerApplyPatch(content string) string {
	idx := strings.Index(content, "[mcp_servers.apply_patch]")
	if idx == -1 {
		return content
	}
	after := content[idx+len("[mcp_servers.apply_patch]"):]
	nextSection := strings.Index(after, "\n[")
	if nextSection == -1 {
		return strings.TrimRight(content[:idx], "\r\n ") + "\n"
	}
	return content[:idx] + after[nextSection+1:]
}

func commentModelProvider(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	modified := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "model_provider") && strings.Contains(trimmed, "=") {
			lines[i] = `# model_provider = "cpa-gui"`
			modified = true
		}
	}
	if modified {
		return strings.Join(lines, "\n"), true
	}
	return content, false
}

func ensureOpenAIBaseURL(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	hasBaseURL := false
	modified := false
	expected := `openai_base_url = "http://127.0.0.1:6868/v1"`

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "openai_base_url") && strings.Contains(trimmed, "=") {
			hasBaseURL = true
			if trimmed != expected {
				lines[i] = expected
				modified = true
			}
			break
		}
	}

	if !hasBaseURL {
		newLines := make([]string, 0, len(lines)+1)
		inserted := false
		for _, line := range lines {
			newLines = append(newLines, line)
			trimmed := strings.TrimSpace(line)
			if !inserted && strings.HasPrefix(trimmed, "model") && strings.Contains(trimmed, "=") {
				newLines = append(newLines, expected)
				inserted = true
			}
		}
		if !inserted {
			newLines = append([]string{expected}, lines...)
		}
		return strings.Join(newLines, "\n"), true
	}

	if modified {
		return strings.Join(lines, "\n"), true
	}
	return content, false
}

func ensureCodexEnvironment() {
	defer func() {
		if r := recover(); r != nil {
			debugLog("ensureCodexEnvironment PANIC RECOVERED: %v", r)
		}
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	codexDir := filepath.Join(home, ".codex")
	configTomlPath := filepath.Join(codexDir, "config.toml")
	authJsonPath := filepath.Join(codexDir, "auth.json")
	catalogJsonPath := filepath.Join(codexDir, "cpa-gui-model-catalog.json")

	// 1. 自动对齐 config.toml
	if contentBytes, err := os.ReadFile(configTomlPath); err == nil {
		content := string(contentBytes)
		c1, m1 := commentModelProvider(content)
		c2, m2 := ensureOpenAIBaseURL(c1)
		c3 := cleanMcpServerApplyPatch(c2)
		m3 := c3 != c2
		if m1 || m2 || m3 {
			_ = os.WriteFile(configTomlPath, []byte(c3), 0644)
			debugLog("ensureCodexEnvironment: auto-aligned config.toml")
		}
	}

	// 2. 自动对齐 auth.json (保留 ChatGPT OAuth 额度条与头像)
	if authBytes, err := os.ReadFile(authJsonPath); err == nil {
		var authData map[string]any
		if err := json.Unmarshal(authBytes, &authData); err == nil {
			modified := false
			if tokens, ok := authData["tokens"].(map[string]any); ok && len(tokens) > 0 {
				if at, ok := tokens["access_token"].(string); ok && strings.TrimSpace(at) != "" {
					if authData["auth_mode"] != "chatgpt" {
						authData["auth_mode"] = "chatgpt"
						modified = true
					}
					if _, hasKey := authData["OPENAI_API_KEY"]; hasKey {
						delete(authData, "OPENAI_API_KEY")
						modified = true
					}
				}
			}
			if modified {
				if out, err := json.MarshalIndent(authData, "", "  "); err == nil {
					_ = os.WriteFile(authJsonPath, append(out, '\n'), 0644)
					debugLog("ensureCodexEnvironment: auto-aligned auth.json (chatgpt oauth mode)")
				}
			}
		}
	}

	// 3. 自动对齐 cpa-gui-model-catalog.json (开启 freeform 补丁能力)
	if catBytes, err := os.ReadFile(catalogJsonPath); err == nil {
		var catData map[string]any
		if err := json.Unmarshal(catBytes, &catData); err == nil {
			modified := false
			if models, ok := catData["models"].([]any); ok {
				for _, m := range models {
					if mObj, ok := m.(map[string]any); ok {
						if mObj["apply_patch_tool_type"] != "freeform" {
							mObj["apply_patch_tool_type"] = "freeform"
							modified = true
						}
					}
				}
			}
			if modified {
				if out, err := json.MarshalIndent(catData, "", "  "); err == nil {
					_ = os.WriteFile(catalogJsonPath, append(out, '\n'), 0644)
					debugLog("ensureCodexEnvironment: auto-aligned cpa-gui-model-catalog.json (freeform)")
				}
			}
		}
	}
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) (retCode C.int) {
	defer func() {
		if r := recover(); r != nil {
			debugLog("cliproxyPluginCall PANIC RECOVERED: %v", r)
			retCode = 0
		}
	}()
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handleMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	debugLog("handleMethod: %s (reqLen=%d)", method, len(request))
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return okEnvelope(registration{
			SchemaVersion: pluginabi.SchemaVersion,
			Metadata: pluginapi.Metadata{
				Name:             "apply_patch",
				Version:          "1.2.0",
				Author:           "codex-agent",
				GitHubRepository: "https://github.com/router-for-me/CLIProxyAPI",
				Logo:             "",
				ConfigFields:     []pluginapi.ConfigField{},
			},
			Capabilities: registrationCapability{
				RequestInterceptor:     true,
				RequestNormalizer:      true,
				StreamChunkInterceptor: true,
				ResponseInterceptor:    true,
				FrontendAuthProvider:   true,
			},
		})
	case pluginabi.MethodFrontendAuthIdentifier:
		return okEnvelope(identifierResponse{Identifier: "apply_patch_auth"})
	case pluginabi.MethodFrontendAuthAuthenticate:
		return handleFrontendAuthAuthenticate(request)
	case pluginabi.MethodRequestInterceptBefore:
		return handleRequestInterceptBefore(request)
	case pluginabi.MethodRequestNormalize:
		return handleRequestNormalize(request)
	case pluginabi.MethodResponseInterceptStreamChunk:
		return handleStreamChunkIntercept(request)
	case pluginabi.MethodResponseInterceptAfter:
		return handleResponseInterceptAfter(request)
	case pluginabi.MethodPluginShutdown:
		return okEnvelope(map[string]any{})
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func handleFrontendAuthAuthenticate(request []byte) ([]byte, error) {
	var req pluginapi.FrontendAuthRequest
	if err := json.Unmarshal(request, &req); err != nil {
		debugLog("handleFrontendAuthAuthenticate: unmarshal err: %v", err)
		return okEnvelope(pluginapi.FrontendAuthResponse{Authenticated: false})
	}

	authHeader := req.Headers.Get("Authorization")
	if authHeader == "" {
		authHeader = req.Headers.Get("X-Api-Key")
	}
	if authHeader == "" {
		authHeader = req.Headers.Get("X-Goog-Api-Key")
	}
	if authHeader == "" && len(req.Query) > 0 {
		authHeader = req.Query.Get("key")
	}

	if authHeader == "" {
		debugLog("handleFrontendAuthAuthenticate: empty auth on %s %s -> rejected", req.Method, req.Path)
		return okEnvelope(pluginapi.FrontendAuthResponse{Authenticated: false})
	}

	token := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	if token == "" {
		debugLog("handleFrontendAuthAuthenticate: empty token on %s %s -> rejected", req.Method, req.Path)
		return okEnvelope(pluginapi.FrontendAuthResponse{Authenticated: false})
	}

	prefixLen := 10
	if len(token) < prefixLen {
		prefixLen = len(token)
	}
	debugLog("handleFrontendAuthAuthenticate: accepted auth for %s %s (token prefix=%s...)", req.Method, req.Path, token[:prefixLen])

	return okEnvelope(pluginapi.FrontendAuthResponse{
		Authenticated: true,
		Principal:     "codex-relay",
		Metadata: map[string]string{
			"provider": "apply_patch_auth",
			"mode":     "relay",
		},
	})
}

// handleRequestInterceptBefore runs BEFORE any translation.
// 1. Converts type: "custom", name: "apply_patch" tool definitions into standard function tools.
// 2. Converts previous turn's custom_tool_call & custom_tool_call_output items to function_call/output.
func handleRequestInterceptBefore(raw []byte) ([]byte, error) {
	var req pluginapi.RequestInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if len(req.Body) == 0 {
		return okEnvelope(pluginapi.RequestInterceptResponse{})
	}

	body := req.Body
	root := gjson.ParseBytes(body)
	modified := false

	// Convert tools[type == "custom" && name == "apply_patch"] -> function tool
	if tools := root.Get("tools"); tools.Exists() && tools.IsArray() {
		tools.ForEach(func(i, tool gjson.Result) bool {
			toolType := tool.Get("type").String()
			name := tool.Get("name").String()
			if toolType == "custom" && name == "apply_patch" {
				path := fmt.Sprintf("tools.%d", i.Int())
				body, _ = sjson.SetBytes(body, path+".type", "function")
				body, _ = sjson.SetBytes(body, path+".name", "apply_patch")
				body, _ = sjson.SetBytes(body, path+".description", applyPatchChatGuidance)
				params := `{"type":"object","properties":{"input":{"type":"string","description":"A V4A patch starting with *** Begin Patch and ending with *** End Patch."}},"required":["input"]}`
				body, _ = sjson.SetRawBytes(body, path+".parameters", []byte(params))
				modified = true
			}
			return true
		})
	}

	// Normalize input history items
	if input := root.Get("input"); input.Exists() && input.IsArray() {
		input.ForEach(func(i, item gjson.Result) bool {
			itemType := item.Get("type").String()
			path := fmt.Sprintf("input.%d", i.Int())
			if itemType == "custom_tool_call" {
				name := item.Get("name").String()
				callID := item.Get("call_id").String()
				inputText := item.Get("input").String()
				argsJSON, _ := sjson.SetBytes([]byte("{}"), "input", inputText)

				body, _ = sjson.SetBytes(body, path+".type", "function_call")
				body, _ = sjson.SetBytes(body, path+".name", name)
				body, _ = sjson.SetBytes(body, path+".call_id", callID)
				body, _ = sjson.SetBytes(body, path+".arguments", string(argsJSON))
				body, _ = sjson.DeleteBytes(body, path+".input")
				modified = true
			} else if itemType == "custom_tool_call_output" {
				body, _ = sjson.SetBytes(body, path+".type", "function_call_output")
				modified = true
			}
			return true
		})
	}

	if modified {
		return okEnvelope(pluginapi.RequestInterceptResponse{Body: body})
	}
	return okEnvelope(pluginapi.RequestInterceptResponse{})
}

// handleRequestNormalize runs AFTER translation to Gemini / Antigravity format.
func handleRequestNormalize(raw []byte) ([]byte, error) {
	var req pluginapi.RequestTransformRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if len(req.Body) == 0 {
		return okEnvelope(pluginapi.PayloadResponse{Body: req.Body})
	}

	toFormat := strings.ToLower(strings.TrimSpace(req.ToFormat))
	isGemini := toFormat == "gemini" || toFormat == "gemini-interactions"
	isAntigravity := toFormat == "antigravity"
	if !isGemini && !isAntigravity {
		return okEnvelope(pluginapi.PayloadResponse{Body: req.Body})
	}

	body := req.Body
	root := gjson.ParseBytes(body)

	toolsPrefix := "tools"
	sysPrefix := "systemInstruction"
	if isAntigravity && root.Get("request").Exists() {
		toolsPrefix = "request.tools"
		sysPrefix = "request.systemInstruction"
	}

	hasApplyPatch := false
	if tools := root.Get(toolsPrefix); tools.Exists() && tools.IsArray() {
		tools.ForEach(func(_, tool gjson.Result) bool {
			if fds := tool.Get("functionDeclarations"); fds.Exists() && fds.IsArray() {
				fds.ForEach(func(_, fd gjson.Result) bool {
					if fd.Get("name").String() == "apply_patch" {
						hasApplyPatch = true
						return false
					}
					return true
				})
			}
			return !hasApplyPatch
		})
	}

	if !hasApplyPatch {
		patchTool := []byte(`{
			"name": "apply_patch",
			"description": "` + jsonEscapeString(applyPatchChatGuidance) + `",
			"parametersJsonSchema": {
				"type": "object",
				"properties": {
					"input": {
						"type": "string",
						"description": "Unified diff patch content starting with '*** Begin Patch' and ending with '*** End Patch'."
					}
				},
				"required": ["input"]
			}
		}`)

		if root.Get(toolsPrefix + ".0.functionDeclarations").Exists() {
			body, _ = sjson.SetRawBytes(body, toolsPrefix+".0.functionDeclarations.-1", patchTool)
		} else {
			newTools := []byte(`[{"functionDeclarations":[]}]`)
			newTools, _ = sjson.SetRawBytes(newTools, "0.functionDeclarations.0", patchTool)
			body, _ = sjson.SetRawBytes(body, toolsPrefix, newTools)
		}
	}

	notice := "\n\nCRITICAL CODING INSTRUCTION:\nWhen editing, creating, or deleting files, you MUST invoke the `apply_patch` function call with the patch content in the `input` argument. Format strictly starting with '*** Begin Patch' and ending with '*** End Patch'."
	if root.Get(sysPrefix + ".parts").Exists() {
		body, _ = sjson.SetBytes(body, sysPrefix+".parts.-1.text", notice)
	} else {
		sys := []byte(fmt.Sprintf(`{"parts":[{"text":%s}]}`, jsonEscape(notice)))
		body, _ = sjson.SetRawBytes(body, sysPrefix, sys)
	}

	return okEnvelope(pluginapi.PayloadResponse{Body: body})
}

// handleStreamChunkIntercept rewrites function_call to the exact 5-event custom_tool_call wire
// expected by Codex CLI's native apply_patch handler.
func handleStreamChunkIntercept(raw []byte) ([]byte, error) {
	var req pluginapi.StreamChunkInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if len(req.Body) == 0 {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{})
	}

	event, data, ok := parseSSEChunk(req.Body)
	if !ok {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{})
	}

	root := gjson.ParseBytes(data)

	switch event {
	case "response.output_item.added":
		item := root.Get("item")
		name := item.Get("name").String()
		if name == "apply_patch" {
			callID := item.Get("call_id").String()
			fcID := item.Get("id").String()
			if fcID == "" {
				fcID = "fc_" + callID
			}
			setPatchCall(callID, fcID)

			// Rewrite to custom_tool_call open frame
			data, _ = sjson.SetBytes(data, "item.type", "custom_tool_call")
			data, _ = sjson.SetBytes(data, "item.id", fcID)
			data, _ = sjson.SetBytes(data, "item.call_id", callID)
			data, _ = sjson.SetBytes(data, "item.name", "apply_patch")
			data, _ = sjson.SetBytes(data, "item.input", "")
			data, _ = sjson.SetBytes(data, "item.status", "in_progress")
			data, _ = sjson.DeleteBytes(data, "item.arguments")

			return okEnvelope(pluginapi.StreamChunkInterceptResponse{
				Body: formatSSEChunk(event, data),
			})
		}

	case "response.function_call_arguments.delta":
		itemID := root.Get("item_id").String()
		callID := strings.TrimPrefix(itemID, "fc_")
		if isPatchCall(callID) {
			// Drop function_call_arguments.delta chunk to avoid client schema errors.
			return okEnvelope(pluginapi.StreamChunkInterceptResponse{
				DropChunk: true,
			})
		}

	case "response.function_call_arguments.done":
		itemID := root.Get("item_id").String()
		callID := strings.TrimPrefix(itemID, "fc_")
		if isPatchCall(callID) {
			fcID := getFcID(callID)
			argsRaw := root.Get("arguments").String()
			input := extractApplyPatchInput(argsRaw)
			recordPatch(callID, input)

			seq := root.Get("sequence_number").Int()
			outputIdx := root.Get("output_index").Int()

			// 1. Emit custom_tool_call_input.delta
			deltaData := []byte(`{"type":"response.custom_tool_call_input.delta","sequence_number":0,"item_id":"","output_index":0,"call_id":"","delta":""}`)
			deltaData, _ = sjson.SetBytes(deltaData, "sequence_number", seq)
			deltaData, _ = sjson.SetBytes(deltaData, "item_id", fcID)
			deltaData, _ = sjson.SetBytes(deltaData, "output_index", outputIdx)
			deltaData, _ = sjson.SetBytes(deltaData, "call_id", callID)
			deltaData, _ = sjson.SetBytes(deltaData, "delta", input)

			// 2. Emit custom_tool_call_input.done
			doneData := []byte(`{"type":"response.custom_tool_call_input.done","sequence_number":0,"item_id":"","output_index":0,"call_id":"","input":""}`)
			doneData, _ = sjson.SetBytes(doneData, "sequence_number", seq+1)
			doneData, _ = sjson.SetBytes(doneData, "item_id", fcID)
			doneData, _ = sjson.SetBytes(doneData, "output_index", outputIdx)
			doneData, _ = sjson.SetBytes(doneData, "call_id", callID)
			doneData, _ = sjson.SetBytes(doneData, "input", input)

			combined := append(formatSSEChunk("response.custom_tool_call_input.delta", deltaData), formatSSEChunk("response.custom_tool_call_input.done", doneData)...)
			return okEnvelope(pluginapi.StreamChunkInterceptResponse{
				Body: combined,
			})
		}

	case "response.output_item.done":
		item := root.Get("item")
		name := item.Get("name").String()
		callID := item.Get("call_id").String()
		if name == "apply_patch" || isPatchCall(callID) {
			fcID := getFcID(callID)
			input := getPatch(callID)
			if input == "" {
				input = extractApplyPatchInput(item.Get("arguments").String())
			}

			data, _ = sjson.SetBytes(data, "item.type", "custom_tool_call")
			data, _ = sjson.SetBytes(data, "item.id", fcID)
			data, _ = sjson.SetBytes(data, "item.call_id", callID)
			data, _ = sjson.SetBytes(data, "item.name", "apply_patch")
			data, _ = sjson.SetBytes(data, "item.input", input)
			data, _ = sjson.SetBytes(data, "item.status", "completed")
			data, _ = sjson.DeleteBytes(data, "item.arguments")

			return okEnvelope(pluginapi.StreamChunkInterceptResponse{
				Body: formatSSEChunk(event, data),
			})
		}

	case "response.completed":
		if outputs := root.Get("response.output"); outputs.Exists() && outputs.IsArray() {
			modified := false
			outputs.ForEach(func(i, item gjson.Result) bool {
				name := item.Get("name").String()
				callID := item.Get("call_id").String()
				if name == "apply_patch" || isPatchCall(callID) {
					path := fmt.Sprintf("response.output.%d", i.Int())
					fcID := getFcID(callID)
					input := getPatch(callID)
					if input == "" {
						input = extractApplyPatchInput(item.Get("arguments").String())
					}
					data, _ = sjson.SetBytes(data, path+".type", "custom_tool_call")
					data, _ = sjson.SetBytes(data, path+".id", fcID)
					data, _ = sjson.SetBytes(data, path+".call_id", callID)
					data, _ = sjson.SetBytes(data, path+".name", "apply_patch")
					data, _ = sjson.SetBytes(data, path+".input", input)
					data, _ = sjson.SetBytes(data, path+".status", "completed")
					data, _ = sjson.DeleteBytes(data, path+".arguments")
					modified = true
				}
				return true
			})
			if modified {
				return okEnvelope(pluginapi.StreamChunkInterceptResponse{
					Body: formatSSEChunk(event, data),
				})
			}
		}
	}

	return okEnvelope(pluginapi.StreamChunkInterceptResponse{})
}

func handleResponseInterceptAfter(raw []byte) ([]byte, error) {
	var req pluginapi.ResponseInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	if len(req.Body) == 0 {
		return okEnvelope(pluginapi.ResponseInterceptResponse{})
	}

	body := req.Body
	root := gjson.ParseBytes(body)
	if outputs := root.Get("output"); outputs.Exists() && outputs.IsArray() {
		modified := false
		outputs.ForEach(func(i, item gjson.Result) bool {
			if item.Get("name").String() == "apply_patch" {
				callID := item.Get("call_id").String()
				fcID := item.Get("id").String()
				if fcID == "" {
					fcID = "fc_" + callID
				}
				input := extractApplyPatchInput(item.Get("arguments").String())
				path := fmt.Sprintf("output.%d", i.Int())
				body, _ = sjson.SetBytes(body, path+".type", "custom_tool_call")
				body, _ = sjson.SetBytes(body, path+".id", fcID)
				body, _ = sjson.SetBytes(body, path+".call_id", callID)
				body, _ = sjson.SetBytes(body, path+".name", "apply_patch")
				body, _ = sjson.SetBytes(body, path+".input", input)
				body, _ = sjson.SetBytes(body, path+".status", "completed")
				body, _ = sjson.DeleteBytes(body, path+".arguments")
				modified = true
			}
			return true
		})
		if modified {
			return okEnvelope(pluginapi.ResponseInterceptResponse{Body: body})
		}
	}

	return okEnvelope(pluginapi.ResponseInterceptResponse{})
}

func extractApplyPatchInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var candidate string
	if gjson.Valid(trimmed) {
		parsed := gjson.Parse(trimmed)
		if v := parsed.Get("input"); v.Exists() && v.Type == gjson.String {
			candidate = v.String()
		} else {
			for _, key := range []string{"patch", "diff", "apply_patch", "input_text", "content"} {
				if v := parsed.Get(key); v.Exists() && v.Type == gjson.String && strings.Contains(v.String(), "*** Begin Patch") {
					candidate = v.String()
					break
				}
			}
		}
	}

	if candidate == "" {
		if strings.Contains(trimmed, "*** Begin Patch") {
			candidate = trimmed
		} else {
			return raw
		}
	}

	return repairV4AEnvelope(candidate)
}

func repairV4AEnvelope(body string) string {
	lines := strings.Split(body, "\n")
	begin := -1
	end := -1
	for i, l := range lines {
		t := strings.TrimRight(l, "\r ")
		if t == "*** Begin Patch" && begin == -1 {
			begin = i
		}
		if t == "*** End Patch" {
			end = i
		}
	}
	if begin == -1 || end == -1 || begin >= end {
		return body
	}
	return strings.Join(lines[begin:end+1], "\n")
}

func parseSSEChunk(payload []byte) (event string, data []byte, ok bool) {
	lines := bytes.Split(payload, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("event:")) {
			event = string(bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("event:"))))
		} else if bytes.HasPrefix(trimmed, []byte("data:")) {
			data = bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
		}
	}
	if event != "" && len(data) > 0 {
		return event, data, true
	}
	return "", nil, false
}

func formatSSEChunk(event string, data []byte) []byte {
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(data)))
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func jsonEscapeString(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}

func okEnvelope(v any) ([]byte, error) {
	raw, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}
