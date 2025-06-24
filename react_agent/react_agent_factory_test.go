package react_agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestGetReactAgentWithAllTools_BeijingWeather(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{} // 这里假设Config结构体和配置加载逻辑可用

	agent, err := GetReactAgentWithAllTools(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	msg, err := agent.Generate(ctx, []*schema.Message{
		{
			Role:    "user",
			Content: "北京温度",
		},
	})

	if err != nil {
		t.Fatalf("failed to generate message: %v", err)
	}

	fmt.Println(msg.Content)
}
