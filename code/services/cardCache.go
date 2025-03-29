package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	maxRetries = 3
	poolSize   = 20
)

// CardPool 管理预创建的卡片实体ID池
type CardPool struct {
	cardQueue     []string      // 卡片ID队列
	mu            sync.Mutex    // 互斥锁保护队列操作
	createCardFn  func(context.Context, string) (string, error) // 创建卡片的函数
	defaultContent string       // 默认卡片内容
}

// NewCardPool 创建一个新的卡片池
func NewCardPool(createFn func(context.Context, string) (string, error), defaultContent string) *CardPool {
	return &CardPool{
		cardQueue:     make([]string, 0, poolSize),
		createCardFn:  createFn,
		defaultContent: defaultContent,
	}
}

// Initialize 初始化卡片池
func (p *CardPool) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// 初始填充池到指定大小
	for i := 0; i < poolSize; i++ {
		cardID, err := p.createCardWithRetry(ctx)
		if err != nil {
			return fmt.Errorf("初始化卡片池失败: %w", err)
		}
		p.cardQueue = append(p.cardQueue, cardID)
		log.Printf("已添加卡片到池中: %s, 当前池大小: %d", cardID, len(p.cardQueue))
	}
	
	return nil
}

// GetCard 从池中获取一个卡片ID，如果池为空则创建新卡片
func (p *CardPool) GetCard(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// 如果队列不为空，从队首取出一个卡片ID
	if len(p.cardQueue) > 0 {
		cardID := p.cardQueue[0]
		p.cardQueue = p.cardQueue[1:] // 移除队首元素
		
		// 异步创建新卡片补充到队列
		go p.asyncReplenishCard(context.Background())
		
		log.Printf("从卡片池获取卡片: %s, 剩余卡片数: %d", cardID, len(p.cardQueue))
		return cardID, nil
	}
	
	// 队列为空，立即创建一个新卡片
	log.Printf("卡片池为空，正在创建新卡片...")
	cardID, err := p.createCardWithRetry(ctx)
	if err != nil {
		return "", fmt.Errorf("创建卡片失败: %w", err)
	}
	
	// 异步创建新卡片补充到队列
	go p.asyncReplenishCard(context.Background())
	
	log.Printf("成功创建新卡片: %s", cardID)
	return cardID, nil
}

// createCardWithRetry 创建卡片并在失败时重试
func (p *CardPool) createCardWithRetry(ctx context.Context) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		cardID, err := p.createCardFn(ctx, p.defaultContent)
		if err == nil {
			return cardID, nil
		}
		lastErr = err
		log.Printf("创建卡片失败(重试 %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Second * time.Duration(i+1)) // 简单的退避策略
	}
	return "", fmt.Errorf("创建卡片重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// asyncReplenishCard 异步创建新卡片并添加到队列
func (p *CardPool) asyncReplenishCard(ctx context.Context) {
	cardID, err := p.createCardWithRetry(ctx)
	if err != nil {
		log.Printf("异步创建卡片失败: %v", err)
		return
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// 确保队列不会超过最大大小
	if len(p.cardQueue) < poolSize {
		p.cardQueue = append(p.cardQueue, cardID)
		log.Printf("异步添加卡片到池中: %s, 当前池大小: %d", cardID, len(p.cardQueue))
	}
}

// GetPoolSize 获取当前池大小
func (p *CardPool) GetPoolSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.cardQueue)
}

// 全局卡片池实例
var globalCardPool *CardPool
var initCardPoolOnce sync.Once

// GetCardPool 获取全局卡片池实例
func GetCardPool() *CardPool {
	return globalCardPool
}

// InitCardPool 初始化全局卡片池
func InitCardPool(ctx context.Context, createFn func(context.Context, string) (string, error), defaultContent string) error {
	var initErr error
	
	initCardPoolOnce.Do(func() {
		pool := NewCardPool(createFn, defaultContent)
		err := pool.Initialize(ctx)
		if err != nil {
			initErr = err
			return
		}
		
		globalCardPool = pool
		log.Printf("全局卡片池已初始化，初始大小: %d", pool.GetPoolSize())
	})
	
	return initErr
}
