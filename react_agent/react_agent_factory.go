package react_agent

import (
	"context"
	"log"

	mcptool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func GetReactAgentWithAllTools(ctx context.Context, cfg *Config) (*react.Agent, error) {

	// 1. 初始化 ChatModel（假设已正确配置）
	cfg, err := LoadConfig("./conf/model_conf.json")
	if err != nil {
		panic(err)
	}
	toolableChatModel, err := cfg.GetChatModel("Doubao-pro-4k")
	if err != nil {
		panic(err)
	}

	// 2. 初始化 MCP 客户端
	mcpServer, err := cfg.GetMCPServer("gaode")
	if err != nil {
		log.Printf("Warning: Failed to get MCP server config: %v", err)
	} else {
		log.Printf("Initializing MCP client for server: %s", mcpServer.Name)
	}

	var mcpTools []tool.BaseTool
	if mcpServer != nil {
		// 创建 SSE MCP 客户端
		cli, err := client.NewSSEMCPClient(mcpServer.Host)
		if err != nil {
			log.Printf("Warning: Failed to create MCP client: %v", err)
		} else {
			// 启动异步通信
			err = cli.Start(ctx)
			if err != nil {
				log.Printf("Warning: Failed to start MCP client: %v", err)
			} else {
				// 初始化客户端
				initRequest := mcp.InitializeRequest{}
				initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
				initRequest.Params.ClientInfo = mcp.Implementation{
					Name:    "eino-mcp-client",
					Version: "1.0.0",
				}
				_, err = cli.Initialize(ctx, initRequest)
				if err != nil {
					log.Printf("Warning: Failed to initialize MCP client: %v", err)
				} else {
					// 获取 MCP 工具
					mcpTools, err = mcptool.GetTools(ctx, &mcptool.Config{Cli: cli})
					if err != nil {
						log.Printf("Warning: Failed to get MCP tools: %v", err)
					} else {
						log.Printf("Successfully got MCP tools")
					}
				}
			}
		}
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: toolableChatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: mcpTools,
		},
	})
	if err != nil {
		panic(err)
	}

	return agent, nil
}
