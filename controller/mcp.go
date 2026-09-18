package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// MCP JSON-RPC 请求结构
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCP JSON-RPC 响应结构
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP 工具定义
type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// research 工具参数 schema
var researchToolSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"stage": map[string]interface{}{
			"type":        "string",
			"description": "科研阶段：literature(文献调研), experiment(实验设计), analysis(数据分析), writing(论文撰写), coding(代码实现), review(综述写作)",
			"enum":        []string{"literature", "experiment", "analysis", "writing", "coding", "review"},
		},
		"message": map[string]interface{}{
			"type":        "string",
			"description": "用户的问题或需求描述",
		},
	},
	"required": []string{"stage", "message"},
}

var researchTools = []MCPTool{
	{
		Name:        "research_chat",
		Description: "科研助手对话工具，支持文献调研、实验设计、数据分析、论文撰写、代码实现、综述写作等科研场景。根据选择的阶段自动使用最合适的模型和系统提示词。",
		InputSchema: researchToolSchema,
	},
	{
		Name:        "research_stages",
		Description: "获取所有可用的科研阶段列表及其描述",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	},
}

// MCPHandler 处理 MCP 协议请求
func MCPHandler(c *gin.Context) {
	var req MCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MCPResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &MCPError{Code: -32700, Message: "Parse error: " + err.Error()},
		})
		return
	}

	switch req.Method {
	case "tools/list":
		handleToolsList(c, req)
	case "tools/call":
		handleToolsCall(c, req)
	case "initialize":
		handleInitialize(c, req)
	default:
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Method not found: " + req.Method},
		})
	}
}

func handleInitialize(c *gin.Context, req MCPRequest) {
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    "research-mcp-server",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		},
	})
}

func handleToolsList(c *gin.Context, req MCPRequest) {
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": researchTools,
		},
	})
}

func handleToolsCall(c *gin.Context, req MCPRequest) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid params: " + err.Error()},
		})
		return
	}

	switch params.Name {
	case "research_stages":
		handleResearchStages(c, req)
	case "research_chat":
		handleResearchChatMCP(c, req, params.Arguments)
	default:
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Unknown tool: " + params.Name},
		})
	}
}

func handleResearchStages(c *gin.Context, req MCPRequest) {
	stages := make([]map[string]interface{}, 0)
	for _, s := range ResearchStages {
		stages = append(stages, map[string]interface{}{
			"key":         s.Key,
			"name":        s.Name,
			"description": s.Description,
			"model":       getResearchModel(s.Key),
		})
	}
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": mustJSON(stages),
				},
			},
		},
	})
}

func handleResearchChatMCP(c *gin.Context, req MCPRequest, args map[string]interface{}) {
	stage, _ := args["stage"].(string)
	message, _ := args["message"].(string)

	if stage == "" || message == "" {
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "stage and message are required"},
		})
		return
	}

	// 查找阶段配置
	var stageConfig *ResearchStage
	for _, s := range ResearchStages {
		if s.Key == stage {
			stageConfig = &s
			break
		}
	}
	if stageConfig == nil {
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid stage: " + stage},
		})
		return
	}

	// 构建请求体
	modelName := getResearchModel(stage)
	openaiReq := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": message},
		},
		"stream": false,
	}

	bodyBytes, _ := json.Marshal(openaiReq)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.URL.Path = "/v1/chat/completions"

	c.Set("original_model", modelName)
	c.Set("research_stage", stageConfig.Key)
	c.Set("research_strategy", stageConfig.Strategy)

	// 使用 response writer 包装器捕获响应
	w := &responseCapture{ResponseWriter: c.Writer, body: new(bytes.Buffer)}
	c.Writer = w

	Relay(c, types.RelayFormatOpenAI)

	// 解析 OpenAI 响应并转换为 MCP 格式
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(w.body.Bytes(), &openaiResp); err != nil {
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": w.body.String()},
				},
			},
		})
		return
	}

	if openaiResp.Error != nil {
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Error: " + openaiResp.Error.Message},
				},
			},
		})
		return
	}

	content := ""
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
	}

	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": content},
			},
		},
	})
}

// responseCapture 包装 gin.ResponseWriter 以捕获响应体
type responseCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseCapture) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func getResearchModel(stage string) string {
	switch stage {
	case "literature":
		return "research-literature"
	case "experiment", "analysis", "coding":
		return "research-experiment"
	case "writing", "review":
		return "research-writing"
	default:
		return "research-literature"
	}
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
