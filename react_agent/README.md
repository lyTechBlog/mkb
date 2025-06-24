# React Agent 使用指南

这是一个基于 [Eino](https://www.cloudwego.io/zh/docs/eino/core_modules/flow_integration_components/react_agent_manual/) 框架实现的最简单的 React Agent。

## 什么是 React Agent？

React Agent 是一个实现了 React 逻辑的智能体框架，它可以让大语言模型拥有"双手"来调用各种工具。React Agent 的核心思想是：

1. **思考 (Reasoning)**: 大模型分析用户输入，决定需要调用什么工具
2. **行动 (Acting)**: 调用相应的工具执行任务
3. **观察 (Observing)**: 观察工具执行的结果
4. **重复**: 根据结果决定是否需要继续调用其他工具

## 项目结构

```
react_agent/
├── ra.go          # React Agent 核心实现
├── chat.go        # 聊天模型配置和基础功能
└── README.md      # 本文件

conf/
└── model_conf.json # 模型配置文件

react_agent_example.go # 使用示例
```

## 功能特性

### 内置工具

1. **示例工具 (example_tool)**
   - 功能：返回固定的示例结果
   - 用途：用于测试和演示

2. **计算器工具 (calculator)**
   - 功能：执行基本的数学运算
   - 支持的运算：加法、减法、乘法、除法
   - 参数：
     - `operation`: 运算类型 ("add", "subtract", "multiply", "divide")
     - `a`: 第一个数字
     - `b`: 第二个数字

## 使用方法

### 1. 基本使用

```go
package main

import (
    "context"
    "fmt"
    "file-upload-server/react_agent"
)

func main() {
    ctx := context.Background()
    
    // 创建 React Agent 并生成响应
    response, err := react_agent.GenerateResponse(ctx, "请计算 10 + 20 的结果")
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    
    fmt.Printf("响应: %s\n", response)
}
```

### 2. 运行示例

```bash
go run react_agent_example.go
```

### 3. 自定义工具

你可以通过实现 `tool.BaseTool` 接口来添加自定义工具：

```go
type MyCustomTool struct {
    Name        string
    Description string
    Execute     func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

func (t *MyCustomTool) GetName() string {
    return t.Name
}

func (t *MyCustomTool) GetDescription() string {
    return t.Description
}

func (t *MyCustomTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
    return t.Execute(ctx, args)
}
```

然后在 `NewReactAgent` 函数中将工具添加到工具列表中：

```go
tools := []tool.BaseTool{
    exampleTool(),
    calculatorTool(),
    &MyCustomTool{
        Name:        "my_tool",
        Description: "我的自定义工具",
        Execute: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
            // 实现你的工具逻辑
            return "工具执行结果", nil
        },
    },
}
```

## 配置说明

### 模型配置

在 `conf/model_conf.json` 中配置你的模型信息：

```json
{
  "models": {
    "gpt-4o": {
      "model_name": "gpt-4o",
      "url": "https://api.openai.com/v1",
      "api_key": "your-api-key-here",
      "prices": {
        "input": 7.5,
        "output": 22.5
      }
    }
  }
}
```

### 环境变量

确保设置了正确的 API 密钥：

```bash
export OPENAI_API_KEY=your-api-key-here
```

## 工作原理

1. **初始化阶段**：
   - 加载模型配置
   - 创建聊天模型实例
   - 注册可用工具

2. **执行阶段**：
   - 接收用户输入
   - 大模型分析是否需要调用工具
   - 如果需要工具，调用相应的工具
   - 将工具结果返回给大模型
   - 重复上述过程直到完成

3. **输出阶段**：
   - 返回最终的响应结果

## 示例对话

### 示例1：使用计算器工具

**用户**: "请计算 15 + 25 的结果"

**Agent 思考过程**:
1. 分析用户需求：需要进行数学计算
2. 选择合适的工具：calculator
3. 调用工具：`calculator(operation="add", a=15, b=25)`
4. 获得结果：40
5. 格式化输出

**响应**: "15 + 25 的结果是 40"

### 示例2：一般对话

**用户**: "你好，请介绍一下你自己"

**Agent 思考过程**:
1. 分析用户需求：不需要调用工具
2. 直接使用大模型生成回复

**响应**: "你好！我是一个基于 Eino 框架的 React Agent..."

## 注意事项

1. **API 密钥安全**: 请确保你的 API 密钥安全，不要将其提交到版本控制系统
2. **错误处理**: 代码中包含了基本的错误处理，但在生产环境中可能需要更完善的错误处理机制
3. **性能优化**: 对于高并发场景，可能需要考虑连接池和缓存机制
4. **工具限制**: 当前实现支持的工具数量有限，可以根据需要扩展

## 扩展建议

1. **添加更多工具**: 如天气查询、翻译、文件操作等
2. **流式输出**: 实现流式响应以提升用户体验
3. **回调机制**: 添加执行过程的回调函数用于监控和日志
4. **多 Agent 协作**: 实现多个 Agent 之间的协作
5. **持久化存储**: 添加对话历史的持久化存储

## 参考文档

- [Eino React Agent 官方文档](https://www.cloudwego.io/zh/docs/eino/core_modules/flow_integration_components/react_agent_manual/)
- [Eino 框架文档](https://www.cloudwego.io/zh/docs/eino/) 