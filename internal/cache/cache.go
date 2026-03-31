package cache

import (
	"sync"
)

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type entry struct {
	key   string
	value interface{}
	prev  *entry
	next  *entry
}

type list struct {
	head *entry
	tail *entry
	size int
}

func newList() *list {
	l := &list{}
	l.head = &entry{}
	l.tail = &entry{}
	l.head.next = l.tail
	l.tail.prev = l.head
	return l
}

type StringCache struct {
	mu       sync.Mutex
	capacity int
	data     map[string]*entry
	order    *list
}

func NewStringCache(capacity int) *StringCache {
	return &StringCache{
		capacity: capacity,
		data:     make(map[string]*entry),
		order:    newList(),
	}
}

func (c *StringCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, ok := c.data[key]
	if !ok {
		return nil, false
	}

	c.moveToFront(ent)
	return ent.value, true
}

func (c *StringCache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, ok := c.data[key]; ok {
		ent.value = value
		c.moveToFront(ent)
		return
	}

	if c.order.size >= c.capacity {
		c.removeOldest()
	}

	ent := &entry{key: key, value: value}
	c.data[key] = ent
	c.addToFront(ent)
}

func (c *StringCache) moveToFront(ent *entry) {
	if ent.prev != nil {
		ent.prev.next = ent.next
		ent.next.prev = ent.prev
		c.order.size--
	}
	c.addToFront(ent)
}

func (c *StringCache) addToFront(ent *entry) {
	ent.next = c.order.head.next
	ent.prev = c.order.head
	c.order.head.next.prev = ent
	c.order.head.next = ent
	c.order.size++
}

func (c *StringCache) removeOldest() {
	if c.order.tail.prev == c.order.head {
		return
	}
	ent := c.order.tail.prev
	ent.prev.next = c.order.tail
	c.order.tail.prev = ent.prev
	delete(c.data, ent.key)
	c.order.size--
}

func (c *StringCache) Values() []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]interface{}, 0, c.order.size)
	for e := c.order.head.next; e != c.order.tail; e = e.next {
		result = append(result, e.value)
	}
	return result
}

var (
	SessionUsage    = NewStringCache(100)
	SessionProject = NewStringCache(1000)
)
