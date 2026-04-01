package agent

import (
	"testing"
	"time"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(2)

	// 测试设置和获取
	entry1 := &ImageCacheEntry{
		Source:    map[string]interface{}{"url": "test1"},
		Timestamp: time.Now().UnixMilli(),
	}
	entry2 := &ImageCacheEntry{
		Source:    map[string]interface{}{"url": "test2"},
		Timestamp: time.Now().UnixMilli(),
	}
	entry3 := &ImageCacheEntry{
		Source:    map[string]interface{}{"url": "test3"},
		Timestamp: time.Now().UnixMilli(),
	}

	cache.Set("key1", entry1)
	cache.Set("key2", entry2)

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	if cache.Get("key1") != entry1 {
		t.Errorf("Expected entry1 for key1")
	}

	// 测试LRU淘汰
	cache.Set("key3", entry3)
	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2 after LRU, got %d", cache.Size())
	}

	if cache.Get("key2") != nil {
		t.Errorf("Expected key2 to be evicted, but it's still in cache")
	}

	// 测试删除
	cache.Delete("key1")
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1 after delete, got %d", cache.Size())
	}

	// 测试清空
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}
}

func TestImageCache(t *testing.T) {
	imgCache := NewImageCache(2)
	source1 := map[string]interface{}{"url": "test1"}
	source2 := map[string]interface{}{"url": "test2"}

	// 测试存储和获取
	imgCache.StoreImage("id1", source1)
	imgCache.StoreImage("id2", source2)

	if !imgCache.HasImage("id1") {
		t.Errorf("Expected id1 to be in cache")
	}

	result := imgCache.GetImage("id1")
	if result == nil {
		t.Errorf("Expected source1 for id1, got nil")
	}
	if result["url"] != source1["url"] {
		t.Errorf("Expected url 'test1', got '%v'", result["url"])
	}

	// 测试缓存过期
	// 这里我们需要修改时间来测试过期，暂时跳过

	// 测试清空
	imgCache.Clear()
	if imgCache.HasImage("id1") {
		t.Errorf("Expected id1 to be removed after clear")
	}
}

func TestGenerateSessionId(t *testing.T) {
	id1 := GenerateSessionId()
	id2 := GenerateSessionId()

	if id1 == "" {
		t.Errorf("Expected non-empty session id")
	}

	if id1 == id2 {
		t.Errorf("Expected different session ids")
	}

	if len(id1) != 16 {
		t.Errorf("Expected session id length 16, got %d", len(id1))
	}
}

func TestRemoveImageMarkers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello [Image #1] world", "Hello  world"},
		{"[Image #123]Test", "Test"},
		{"No markers here", "No markers here"},
		{"Multiple [Image #1] markers [Image #2] here", "Multiple  markers  here"},
	}

	for _, test := range tests {
		result := removeImageMarkers(test.input)
		if result != test.expected {
			t.Errorf("Expected '%s', got '%s' for input '%s'", test.expected, result, test.input)
		}
	}
}
