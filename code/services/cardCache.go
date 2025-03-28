package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// CardPool 管理预创建的卡片实体ID池
type CardPool struct {
	cardQueue     []string      // 卡片ID队列
	mu            sync.Mutex    // 互斥锁保护队列操作
	minPoolSize   int           // 最小池大小
	maxPoolSize   int           // 最大池大小
	createCardFn  func(context.Context, string) (string, error) // 创建卡片的函数
	defaultContent string       // 默认卡片内容
	isRunning     bool          // 标记定时器是否运行中
	stopCh        chan struct{} // 停止定时器的通道
}

// NewCardPool 创建一个新的卡片池
func NewCardPool(
	minSize int, 
	maxSize int, 
	createFn func(context.Context, string) (string, error),
	defaultContent string,
) *CardPool {
	if minSize <= 0 {
		minSize = 10 // 默认最小池大小
	}
	if maxSize <= 0 || maxSize < minSize {
		maxSize = minSize * 2 // 默认最大池大小
	}
	
	return &CardPool{
		cardQueue:     make([]string, 0, maxSize),
		minPoolSize:   minSize,
		maxPoolSize:   maxSize,
		createCardFn:  createFn,
		defaultContent: defaultContent,
		stopCh:        make(chan struct{}),
	}
}

// Initialize 初始化卡片池并启动维护协程
func (p *CardPool) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// 初始填充池
	err := p.fillPool(ctx)
	if err != nil {
		return fmt.Errorf("初始化卡片池失败: %w", err)
	}
	
	// 启动维护协程
	p.startMaintenanceRoutine(ctx)
	
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
		log.Printf("从卡片池获取卡片: %s, 剩余卡片数: %d", cardID, len(p.cardQueue))
		return cardID, nil
	}
	
	// 队列为空，立即创建一个新卡片
	log.Printf("卡片池为空，正在创建新卡片...")
	cardID, err := p.createCardFn(ctx, p.defaultContent)
	if err != nil {
		return "", fmt.Errorf("创建卡片失败: %w", err)
	}
	
	log.Printf("成功创建新卡片: %s", cardID)
	return cardID, nil
}

// fillPool 填充卡片池至最小大小
func (p *CardPool) fillPool(ctx context.Context) error {
	currentSize := len(p.cardQueue)
	neededCards := p.minPoolSize - currentSize
	
	if neededCards <= 0 {
		return nil // 池已满足最小大小要求
	}
	
	log.Printf("填充卡片池，当前大小: %d, 目标大小: %d", currentSize, p.minPoolSize)
	
	// 创建所需数量的卡片
	for i := 0; i < neededCards; i++ {
		cardID, err := p.createCardFn(ctx, p.defaultContent)
		if err != nil {
			return fmt.Errorf("创建卡片失败: %w", err)
		}
		
		// 将新卡片ID添加到队列尾部
		p.cardQueue = append(p.cardQueue, cardID)
		log.Printf("已添加卡片到池中: %s, 当前池大小: %d", cardID, len(p.cardQueue))
	}
	
	return nil
}

// startMaintenanceRoutine 启动维护协程，定期检查并填充池
func (p *CardPool) startMaintenanceRoutine(ctx context.Context) {
	if p.isRunning {
		return // 已经在运行中
	}
	
	p.isRunning = true
	p.stopCh = make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				func() {
					maintenanceCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					
					p.mu.Lock()
					defer p.mu.Unlock()
					
					// 检查池大小并填充
					if len(p.cardQueue) < p.minPoolSize {
						log.Printf("维护协程: 卡片池大小 %d 低于最小值 %d, 正在填充...", len(p.cardQueue), p.minPoolSize)
						err := p.fillPool(maintenanceCtx)
						if err != nil {
							log.Printf("维护协程: 填充卡片池失败: %v", err)
						}
					}
				}()
			case <-p.stopCh:
				log.Printf("维护协程: 停止卡片池维护")
				return
			case <-ctx.Done():
				log.Printf("维护协程: 上下文取消，停止卡片池维护")
				return
			}
		}
	}()
}

// Stop 停止卡片池维护协程
func (p *CardPool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.isRunning {
		close(p.stopCh)
		p.isRunning = false
		log.Printf("卡片池维护已停止")
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
		pool := NewCardPool(10, 20, createFn, defaultContent)
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
