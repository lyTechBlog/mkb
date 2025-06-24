package react_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

type Config struct {
	Models         map[string]ModelConfig     `json:"models"`
	EmbeddingModel ModelConfig                `json:"embedding_model"`
	MCPServers     map[string]MCPServerConfig `json:"mcp_servers"`
}

type ModelConfig struct {
	ModelName string `json:"model_name"`
	URL       string `json:"url"`
	APIKey    string `json:"api_key"`
	Prices    Prices `json:"prices"`
	PS        string `json:"ps,omitempty"` // omitempty表示如果该字段为空，则不进行序列化
}

type MCPServerConfig struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	APIKey string `json:"api_key"`
	Type   string `json:"type"` // "sse" or "stdio"
}

type Prices struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

/**
* 加载配置文件
* @param path 配置文件路径
* @return 配置文件内容
* @return 错误信息
 */
func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path) // 将文件读取为字节切片
	if err != nil {
		return nil, err
	}

	var config Config
	// 将字节切片反序列化为Config结构体
	// unmarshal = un(反) marshal(序列化)
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) GetChatModel(modelName string) (*openai.ChatModel, error) {
	model, exists := c.Models[modelName]
	if !exists {
		return nil, fmt.Errorf("model %s not found in configuration", modelName)
	}

	return openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: model.URL,
		Model:   model.ModelName,
		APIKey:  model.APIKey,
	})
}

func (c *Config) Chat(modelName, question string) (string, error) {
	model, exists := c.Models[modelName]
	fmt.Println(model)
	if !exists {
		return "", fmt.Errorf("model %s not found in configuration", modelName)
	}

	// Create template for chat
	template := prompt.FromMessages(schema.FString,
		schema.SystemMessage("你是一个专业的AI助手。请用专业且友好的语气回答问题。"),
		schema.UserMessage(question),
	)

	// Format messages
	// Learn: 这里使用了标准库的context，以及eino的callbacks，需要再学习
	//Learn: context的实际使用例子，和基本原理学习（基于书籍）
	//Learn: callbacks 的实际使用例子，和基本原理学习（基于书籍）
	messages, err := template.Format(context.Background(), nil)
	fmt.Println(messages)
	if err != nil {
		return "", err
	}

	chatModel, _ := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: model.URL,
		Model:   model.ModelName,
		APIKey:  model.APIKey,
	})

	result, err := chatModel.Generate(context.Background(), messages)
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

func (c *Config) GetMCPServer(serverName string) (*MCPServerConfig, error) {
	server, exists := c.MCPServers[serverName]
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found in configuration", serverName)
	}
	return &server, nil
}
