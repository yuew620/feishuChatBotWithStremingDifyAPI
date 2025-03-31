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

// NewCardPool creates and initializes a new card pool
func NewCardPool(createFn CreateCardFn) *CardPool {
	p := &CardPool{}
	p.Init(createFn)
	return p
}

// Init 初始化卡片池
func (p *CardPool) Init(createFn CreateCardFn) {
	log.Printf("[CardPool] Initializing card pool with target size: %d", PoolSize)
	p.cards = list.New()
	p.createFn = createFn
	p.stopChan = make(chan struct{})

	// 同步初始化卡片池
	log.Printf("[CardPool] ===== Starting initial pool fill with size %d at %v =====", PoolSize, time.Now().Format("15:04:05"))
	startTime := time.Now()
	p.fillPool(context.Background())
	log.Printf("[CardPool] ===== Initial pool fill completed at %v, took %v, current size: %d =====", 
		time.Now().Format("15:04:05"),
		time.Since(startTime),
		p.GetPoolSize())

	// 启动后台任务
	p.startBackgroundTasks()
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
	log.Printf("Rebuilding pool with size %d", PoolSize)
	p.fillPool(ctx)
}

// fillPool 填充卡片池至目标大小
func (p *CardPool) fillPool(ctx context.Context) {
	for {
		p.mu.RLock()
		currentSize := p.cards.Len()
		p.mu.RUnlock()

		if currentSize >= PoolSize {
			log.Printf("[CardPool] Pool filled to target size: %d at %v", PoolSize, time.Now().Format("15:04:05"))
			break
		}

		cardStartTime := time.Now()
		log.Printf("[CardPool] >>>>> Creating card %d/%d at %v", currentSize+1, PoolSize, time.Now().Format("15:04:05"))
		
		// 同步创建新卡片
		if err := p.CreateCardWithRetry(ctx); err != nil {
			log.Printf("[CardPool] !!!!! Failed to create card %d/%d: %v", currentSize+1, PoolSize, err)
			// 继续尝试创建，避免池子逐渐缩小
			continue
		}
		
		log.Printf("[CardPool] <<<<< Card %d/%d created successfully in %v", currentSize+1, PoolSize, time.Since(cardStartTime))

		// 避免创建过快
		time.Sleep(100 * time.Millisecond)
	}
}

// CreateCardWithRetry 创建卡片并进行重试
func (p *CardPool) CreateCardWithRetry(ctx context.Context) error {
	var cardID string
	var err error
	
	// 重试逻辑
	for i := 0; i < MaxRetries; i++ {
		if i > 0 {
			// 重试前等待
			time.Sleep(RetryInterval)
		}

		log.Printf("[CardPool] Attempting to create card (attempt %d/%d) at %v", i+1, MaxRetries, time.Now().Format("15:04:05"))
		cardID, err = p.createFn(ctx)
		if err == nil {
			log.Printf("[CardPool] Successfully created card with ID: %s", cardID)
			break
		}
		log.Printf("[CardPool] Failed to create card (attempt %d/%d): %v", i+1, MaxRetries, err)
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

	log.Printf("[CardPool] Successfully created and added new card to pool: %s at %v", cardID, time.Now().Format("15:04:05"))
	return nil
}

// GetCard 从池中获取一个卡片
func (p *CardPool) GetCard(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查是否有可用卡片
	if p.cards.Len() == 0 {
		log.Printf("[CardPool] No cards available in pool, creating new one at %v", time.Now().Format("15:04:05"))
		// 如果没有可用卡片，使用CreateCardWithRetry创建一个
		if err := p.CreateCardWithRetry(ctx); err != nil {
			return "", fmt.Errorf("failed to create card: %w", err)
		}

		// 获取刚创建的卡片
		element := p.cards.Back()
		p.cards.Remove(element)
		card := element.Value.(*CardEntry)

		// 异步创建一个新卡片补充到池中
		go func() {
			if err := p.CreateCardWithRetry(ctx); err != nil {
				log.Printf("[CardPool] Failed to create replacement card at %v: %v", time.Now().Format("15:04:05"), err)
				// 继续尝试创建，避免池子逐渐缩小
				go p.CreateCardWithRetry(ctx)
			}
		}()

		return card.CardID, nil
	}

	// 获取并移除第一个卡片
	element := p.cards.Front()
	p.cards.Remove(element)
	card := element.Value.(*CardEntry)

	log.Printf("[CardPool] Got card from pool: %s, remaining cards: %d at %v", card.CardID, p.cards.Len(), time.Now().Format("15:04:05"))

	// 异步创建新卡片补充到池中
	go func() {
		if err := p.CreateCardWithRetry(ctx); err != nil {
			log.Printf("Failed to create replacement card: %v", err)
			// 继续尝试创建，避免池子逐渐缩小
			go p.CreateCardWithRetry(ctx)
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
