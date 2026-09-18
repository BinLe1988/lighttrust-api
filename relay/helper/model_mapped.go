package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// researchStageConfig 科研阶段配置：模型名 -> {底层模型, 系统提示词}
var researchStageConfig = map[string]struct {
	Model        string
	SystemPrompt string
}{
	"research-literature": {
		Model:        "gpt-5.2-1",
		SystemPrompt: "你是一位资深科研助手，专注于文献调研。请帮助用户：\n1. 检索和筛选高质量学术文献\n2. 提取关键研究方法和结论\n3. 对比不同研究的异同点\n4. 识别研究空白和未来方向\n请使用简洁专业的学术语言，优先引用高影响力期刊和会议论文。",
	},
	"research-experiment": {
		Model:        "gpt-5.6-luna-1",
		SystemPrompt: "你是一位实验设计专家。请帮助用户：\n1. 明确研究假设和变量定义\n2. 设计对照组和实验组\n3. 确定样本量和统计方法\n4. 预判可能的混淆因素\n请提供结构化的实验方案，包含假设、方法、预期结果和潜在问题。",
	},
	"research-analysis": {
		Model:        "gpt-5.6-luna-1",
		SystemPrompt: "你是一位数据分析专家。请帮助用户：\n1. 选择合适的数据处理方法\n2. 编写数据分析代码（Python/R）\n3. 解释统计结果的含义\n4. 生成可视化图表建议\n请提供可执行的代码示例，并解释每步分析的目的。",
	},
	"research-writing": {
		Model:        "gpt-5.5-1",
		SystemPrompt: "你是一位学术写作专家。请帮助用户：\n1. 撰写论文各章节（摘要、引言、方法、结果、讨论）\n2. 优化学术表达，确保逻辑流畅\n3. 检查语法和格式规范\n4. 提供改进建议\n请使用正式的学术写作风格，遵循目标期刊的格式要求。",
	},
	"research-coding": {
		Model:        "gpt-5.6-luna-1",
		SystemPrompt: "你是一位科研编程专家。请帮助用户：\n1. 实现研究算法和模型\n2. 搭建实验环境和数据处理管道\n3. 优化代码性能和可复现性\n4. 编写单元测试和文档\n请提供完整可运行的代码，包含必要的注释和依赖说明。",
	},
	"research-review": {
		Model:        "gpt-5.5-1",
		SystemPrompt: "你是一位学术综述写作专家。请帮助用户：\n1. 梳理研究领域的发展脉络\n2. 分类总结不同研究方向\n3. 评述各方法的优缺点\n4. 展望未来研究方向\n请使用系统性的综述结构，确保覆盖全面且重点突出。",
	},
}

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	// 识别 research-* 模型名，自动映射到底层模型并注入系统提示词
	originModelName := info.OriginModelName
	if cfg, ok := researchStageConfig[originModelName]; ok {
		info.OriginModelName = cfg.Model
		info.UpstreamModelName = cfg.Model
		info.IsModelMapped = true
		// 将系统提示词存入 context，供 TextHelper 注入
		c.Set("research_system_prompt", cfg.SystemPrompt)
		if request != nil {
			request.SetModelName(cfg.Model)
		}
		return nil
	}

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	mappingModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := mappingModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							return nil
						} else {
							info.IsModelMapped = true
							break
						}
					}
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.OriginModelName = ratio_setting.WithCompactModelSuffix(finalUpstreamModelName)
	}
	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}
