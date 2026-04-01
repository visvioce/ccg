package transformer

import (
	"encoding/json"
)

// OpenAIResponsesRequestTransformer transforms OpenAI Responses API format to standard Chat Completions format
func OpenAIResponsesRequestTransformer(body map[string]any, _ string) map[string]any {
	// Check if this is a Responses API format request
	if _, ok := body["input"]; !ok {
		return body
	}

	// Convert Responses API format to Chat Completions format
	chatBody := make(map[string]any)

	// Copy model
	if model, ok := body["model"].(string); ok {
		chatBody["model"] = model
	}

	// Convert input to messages
	if input, ok := body["input"].(string); ok {
		chatBody["messages"] = []map[string]any{
			{"role": "user", "content": input},
		}
	} else if inputArr, ok := body["input"].([]any); ok {
		var messages []map[string]any
		for _, item := range inputArr {
			if itemMap, ok := item.(map[string]any); ok {
				msg := map[string]any{
					"role":    itemMap["role"],
					"content": itemMap["content"],
				}
				messages = append(messages, msg)
			}
		}
		chatBody["messages"] = messages
	}

	// Copy optional parameters
	if maxTokens, ok := body["max_output_tokens"].(float64); ok {
		chatBody["max_tokens"] = int(maxTokens)
	}
	if temp, ok := body["temperature"].(float64); ok {
		chatBody["temperature"] = temp
	}
	if topP, ok := body["top_p"].(float64); ok {
		chatBody["top_p"] = topP
	}
	if stream, ok := body["stream"].(bool); ok {
		chatBody["stream"] = stream
	}

	// Copy tools if present
	if tools, ok := body["tools"].([]any); ok {
		var chatTools []map[string]any
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]any); ok {
				chatTool := map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":        toolMap["name"],
						"description": toolMap["description"],
						"parameters":  toolMap["parameters"],
					},
				}
				chatTools = append(chatTools, chatTool)
			}
		}
		chatBody["tools"] = chatTools
	}

	return chatBody
}

// OpenAIResponsesResponseTransformer transforms Chat Completions response to Responses API format
func OpenAIResponsesResponseTransformer(respBody []byte) []byte {
	var resp map[string]any
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return respBody
	}

	// Already in Responses API format
	if _, ok := resp["output"]; ok {
		return respBody
	}

	// Convert Chat Completions format to Responses API format
	responsesResp := make(map[string]any)

	// Copy basic fields
	if id, ok := resp["id"].(string); ok {
		responsesResp["id"] = id
	}
	if model, ok := resp["model"].(string); ok {
		responsesResp["model"] = model
	}
	responsesResp["object"] = "response"

	// Convert choices to output
	if choices, ok := resp["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				output := []map[string]any{
					{
						"type":    "message",
						"role":    "assistant",
						"content": message["content"],
					},
				}

				// Handle tool calls
				if toolCalls, ok := message["tool_calls"].([]any); ok && len(toolCalls) > 0 {
					var outputTools []map[string]any
					for _, tc := range toolCalls {
						if tcMap, ok := tc.(map[string]any); ok {
							outputTool := map[string]any{
								"type": "function_call",
								"name": "",
							}
							if fn, ok := tcMap["function"].(map[string]any); ok {
								outputTool["name"] = fn["name"]
								if args, ok := fn["arguments"].(string); ok {
									var argsObj any
									if json.Unmarshal([]byte(args), &argsObj) == nil {
										outputTool["arguments"] = argsObj
									} else {
										outputTool["arguments"] = args
									}
								}
							}
							if id, ok := tcMap["id"].(string); ok {
								outputTool["call_id"] = id
							}
							outputTools = append(outputTools, outputTool)
						}
					}
					if len(outputTools) > 0 {
						output = append(output, outputTools...)
					}
				}

				responsesResp["output"] = output
			}
		}
	}

	// Copy usage
	if usage, ok := resp["usage"].(map[string]any); ok {
		responsesResp["usage"] = usage
	}

	result, _ := json.Marshal(responsesResp)
	return result
}
