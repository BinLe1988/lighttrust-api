package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResearchStage 科研阶段定义
type ResearchStage struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	NameEn      string `json:"name_en"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Strategy    string `json:"strategy"`       // 路由策略
	ModelPrompt string `json:"model_prompt"`   // 推荐模型提示
	SystemPrompt string `json:"system_prompt"` // 系统提示词
}

// ResearchStages 科研阶段配置
var ResearchStages = []ResearchStage{
	{
		Key:         "literature",
		Name:        "文献调研",
		NameEn:      "Literature Review",
		Description: "检索、阅读和总结学术文献，快速掌握研究领域现状",
		Icon:        "book-open",
		Strategy:    "omni-auto:balanced",
		ModelPrompt: "推荐使用强检索和总结能力的模型",
		SystemPrompt: `你是一位资深科研助手，专注于文献调研。请帮助用户：
1. 检索和筛选高质量学术文献
2. 提取关键研究方法和结论
3. 对比不同研究的异同点
4. 识别研究空白和未来方向
请使用简洁专业的学术语言，优先引用高影响力期刊和会议论文。`,
	},
	{
		Key:         "experiment",
		Name:        "实验设计",
		NameEn:      "Experiment Design",
		Description: "设计实验方案、确定变量和评估方法",
		Icon:        "flask",
		Strategy:    "omni-auto:coding",
		ModelPrompt: "推荐使用逻辑推理和结构化输出能力强的模型",
		SystemPrompt: `你是一位实验设计专家。请帮助用户：
1. 明确研究假设和变量定义
2. 设计对照组和实验组
3. 确定样本量和统计方法
4. 预判可能的混淆因素
请提供结构化的实验方案，包含假设、方法、预期结果和潜在问题。`,
	},
	{
		Key:         "analysis",
		Name:        "数据分析",
		NameEn:      "Data Analysis",
		Description: "处理实验数据、统计分析和可视化",
		Icon:        "bar-chart",
		Strategy:    "omni-auto:coding",
		ModelPrompt: "推荐使用数据处理和代码生成能力强的模型",
		SystemPrompt: `你是一位数据分析专家。请帮助用户：
1. 选择合适的数据处理方法
2. 编写数据分析代码（Python/R）
3. 解释统计结果的含义
4. 生成可视化图表建议
请提供可执行的代码示例，并解释每步分析的目的。`,
	},
	{
		Key:         "writing",
		Name:        "论文撰写",
		NameEn:      "Paper Writing",
		Description: "撰写学术论文各章节，优化表达和格式",
		Icon:        "pen",
		Strategy:    "omni-auto:balanced",
		ModelPrompt: "推荐使用高质量文本生成能力的模型",
		SystemPrompt: `你是一位学术写作专家。请帮助用户：
1. 撰写论文各章节（摘要、引言、方法、结果、讨论）
2. 优化学术表达，确保逻辑流畅
3. 检查语法和格式规范
4. 提供改进建议
请使用正式的学术写作风格，遵循目标期刊的格式要求。`,
	},
	{
		Key:         "coding",
		Name:        "代码实现",
		NameEn:      "Code Implementation",
		Description: "实现算法、搭建实验环境和数据处理管道",
		Icon:        "code",
		Strategy:    "omni-auto:coding",
		ModelPrompt: "推荐使用代码生成能力最强的模型",
		SystemPrompt: `你是一位科研编程专家。请帮助用户：
1. 实现研究算法和模型
2. 搭建实验环境和数据处理管道
3. 优化代码性能和可复现性
4. 编写单元测试和文档
请提供完整可运行的代码，包含必要的注释和依赖说明。`,
	},
	{
		Key:         "review",
		Name:        "综述写作",
		NameEn:      "Review Writing",
		Description: "撰写文献综述，梳理研究脉络和前沿进展",
		Icon:        "scroll",
		Strategy:    "omni-auto:balanced",
		ModelPrompt: "推荐使用长文本理解和总结能力强的模型",
		SystemPrompt: `你是一位学术综述写作专家。请帮助用户：
1. 梳理研究领域的发展脉络
2. 分类总结不同研究方向
3. 评述各方法的优缺点
4. 展望未来研究方向
请使用系统性的综述结构，确保覆盖全面且重点突出。`,
	},
}

// GetResearchStages 获取科研阶段列表
func GetResearchStages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    ResearchStages,
	})
}

// GetResearchStage 获取单个科研阶段配置
func GetResearchStage(c *gin.Context) {
	stageKey := c.Param("stage")
	for _, stage := range ResearchStages {
		if stage.Key == stageKey {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "",
				"data":    stage,
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "科研阶段不存在",
	})
}

// ResearchChat 科研场景对话接口
func ResearchChat(c *gin.Context) {
	var req struct {
		Stage   string    `json:"stage" binding:"required"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages" binding:"required"`
		Stream bool `json:"stream"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 查找对应阶段的配置
	var stageConfig *ResearchStage
	for _, stage := range ResearchStages {
		if stage.Key == req.Stage {
			stageConfig = &stage
			break
		}
	}
	if stageConfig == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的科研阶段",
		})
		return
	}

	// 将阶段配置写入上下文，供 relay 模块使用
	c.Set("research_stage", stageConfig.Key)
	c.Set("research_strategy", stageConfig.Strategy)
	c.Set("research_system_prompt", stageConfig.SystemPrompt)

	// 在消息列表前插入系统提示词
	var messages []map[string]string
	messages = append(messages, map[string]string{
		"role":    "system",
		"content": stageConfig.SystemPrompt,
	})
	for _, msg := range req.Messages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// 使用通用 relay 处理
	c.Set("messages", messages)
	c.Set("stream", req.Stream)

	// 调用现有的 relay 逻辑
	RelayResearch(c)
}

// RelayResearch 科研场景专用 relay
func RelayResearch(c *gin.Context) {
	// 复用现有的 relay 基础设施
	// 通过 context 传递科研阶段配置
	Relay(c, "openai")
}
