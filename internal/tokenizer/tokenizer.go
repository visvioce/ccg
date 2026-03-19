package tokenizer

import (
	"encoding/json"
	"sync"
)

var (
	enc *Tiktoken
	mu  sync.Once
)

type Tiktoken struct {
	encoder        map[string]int
	mergeableRanks map[string]int
	explicitNVocab int
}

func Init() error {
	var initErr error
	mu.Do(func() {
		cl100kBase, err := loadTiktoken("cl100k_base")
		if err != nil {
			initErr = err
			return
		}
		enc = cl100kBase
	})
	return initErr
}

func loadTiktoken(model string) (*Tiktoken, error) {
	encoder := make(map[string]int)
	mergeableRanks := make(map[string]int)

	switch model {
	case "cl100k_base":
		for i := 0; i < 256; i++ {
			encoder[string(rune(i))] = i
		}
	default:
		for i := 0; i < 256; i++ {
			encoder[string(rune(i))] = i
		}
	}

	return &Tiktoken{
		encoder:        encoder,
		mergeableRanks: mergeableRanks,
		explicitNVocab: 256,
	}, nil
}

func (t *Tiktoken) Encode(text string) []int {
	if t == nil {
		return []int{}
	}

	var tokens []int
	for _, ch := range text {
		if tok, ok := t.encoder[string(ch)]; ok {
			tokens = append(tokens, tok)
		} else {
			bytes := []byte(string(ch))
			for _, b := range bytes {
				tokens = append(tokens, int(b))
			}
		}
	}
	return tokens
}

func CountTokens(text string) int {
	if enc == nil {
		return len(text)
	}
	return len(enc.Encode(text))
}

type Message struct {
	Role    string
	Content interface{}
}

type Tool struct {
	Name        string
	Description string
	InputSchema interface{}
}

type RequestBody struct {
	Model    string
	Messages []Message
	System   interface{}
	Tools    []Tool
	Thinking interface{}
}

func CountRequestTokens(body *RequestBody) int {
	if enc == nil {
		return 0
	}

	count := 0

	for _, msg := range body.Messages {
		count += 4
		switch content := msg.Content.(type) {
		case string:
			count += CountTokens(content)
		case json.RawMessage:
			count += CountTokens(string(content))
		}
	}

	if body.System != nil {
		count += 3
		switch sys := body.System.(type) {
		case string:
			count += CountTokens(sys)
		case json.RawMessage:
			count += CountTokens(string(sys))
		}
	}

	for _, tool := range body.Tools {
		count += 15
		if tool.Description != "" {
			count += CountTokens(tool.Name + tool.Description)
		}
		if tool.InputSchema != nil {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			count += CountTokens(string(schemaBytes))
		}
	}

	count += 3

	return count
}
