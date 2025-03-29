package cardpool

import (
	"container/list"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	PoolSize        = 20              // 卡片池大小
	CardExpiration  = 24 * time.Hour  // 卡片过期时间
	MaxRetries      = 3               // 最大重试次数
	RetryInterval   = 1 * time.Second // 重试间隔
)

// CardEntry 表示卡片池中的一个卡片条目
type CardEntry struct {
	CardID    string    // 卡片ID
	CreatedAt time.Time // 创建时间
}

// CardPool 卡片池结构
type CardPool struct {
	cards     *list.List    // 卡片链表
	mu        sync.RWMutex  // 保护cards的并发访问
	createFn  CreateCardFn  // 创建卡片的函数
	stopChan  chan struct{} // 用于停止后台任务
	isRunning bool         // 标记后台任务是否运行中
}

// CreateCardFn 定义创建卡片的函数类型
type CreateCardFn func(context.Context) (string, error)

// NewCardPool 创建新的卡片池
func NewCardPool(createFn CreateCardFn) *CardPool {
	pool := &CardPool{
		cards:    list.New(),
		createFn: createFn,
		stopChan: make(chan struct{}),
	}

	// 启动后台任务
	pool.startBackgroundTasks()

	return pool
}

// startBackgroundTasks 启动后台任务
func (p *CardPool) startBackgroundTasks() {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return
	}
	p.isRunning = true
	p.mu.Unlock()

	// 启动定时重建任务
	go p.rebuildAtMidnight()

	// 初始填充卡片池
	go p.fillPool(context.Background())
}

// Stop 停止卡片池的后台任务
func (p *CardPool) Stop() {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return
	}
	p.isRunning = false
	close(p.stopChan)
	p.mu.Unlock()
}

// rebuildAtMidnight 在每天0点重建卡片池
func (p *CardPool) rebuildAtMidnight() {
	for {
		select {
		case <-p.stopChan:
			return
		default:
			now := time.Now()
			next := now.Add(24 * time.Hour)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			duration := next.Sub(now)

			select {
			case <-p.stopChan:
				return
			case <-time.After(duration):
				log.Printf("Starting daily card pool rebuild at %v", time.Now())
				p.RebuildPool(context.Background())
			}
		}
	}
}

// RebuildPool 重建整个卡片池
func (p *CardPool) RebuildPool(ctx context.Context) {
	p.mu.Lock()
	p.cards = list.New() // 清空现有卡片
	p.mu.Unlock()

	// 重新填充池
	p.fillPool(ctx)
}

// fillPool 填充卡片池至目标大小
func (p *CardPool) fillPool(ctx context.Context) {
	for {
		p.mu.RLock()
		currentSize := p.cards.Len()
		p.mu.RUnlock()

		if currentSize >= PoolSize {
			break
		}

		// 异步创建新卡片
		go func() {
			if err := p.createCardWithRetry(ctx); err != nil {
				log.Printf("Failed to create card during pool fill: %v", err)
				// 继续尝试创建，避免池子逐渐缩小
				go p.createCardWithRetry(ctx)
			}
		}()

		// 避免创建过快
		time.Sleep(100 * time.Millisecond)
	}
}

// createCardWithRetry 创建卡片并进行重试
func (p *CardPool) createCardWithRetry(ctx context.Context) error {
	var cardID string
	var err error
	
	// 重试逻辑
	for i := 0; i < MaxRetries; i++ {
		if i > 0 {
			// 重试前等待
			time.Sleep(RetryInterval)
		}

		cardID, err = p.createFn(ctx)
		if err == nil {
			break
		}
		log.Printf("Failed to create card (attempt %d/%d): %v", i+1, MaxRetries, err)
	}

	if err != nil {
		return fmt.Errorf("failed to create card after %d attempts: %w", MaxRetries, err)
	}

	// 将新卡片添加到池中
	p.mu.Lock()
	p.cards.PushBack(&CardEntry{
		CardID:    cardID,
		CreatedAt: time.Now(),
	})
	p.mu.Unlock()

	log.Printf("Successfully created and added new card to pool: %s", cardID)
	return nil
}

// GetCard 从池中获取一个卡片
func (p *CardPool) GetCard(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查是否有可用卡片
	if p.cards.Len() == 0 {
		// 如果没有可用卡片，直接创建一个
		cardID, err := p.createFn(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to create card: %w", err)
		}

		// 异步创建一个新卡片补充到池中
		go func() {
			if err := p.createCardWithRetry(ctx); err != nil {
				log.Printf("Failed to create replacement card: %v", err)
				// 继续尝试创建，避免池子逐渐缩小
				go p.createCardWithRetry(ctx)
			}
		}()

		return cardID, nil
	}

	// 获取并移除第一个卡片
	element := p.cards.Front()
	p.cards.Remove(element)
	card := element.Value.(*CardEntry)

	// 异步创建新卡片补充到池中
	go func() {
		if err := p.createCardWithRetry(ctx); err != nil {
			log.Printf("Failed to create replacement card: %v", err)
			// 继续尝试创建，避免池子逐渐缩小
			go p.createCardWithRetry(ctx)
		}
	}()

	return card.CardID, nil
}

// GetPoolSize 获取当前池中的卡片数量
func (p *CardPool) GetPoolSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cards.Len()
}
