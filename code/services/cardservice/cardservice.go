package cardservice

import (
	"log"
	"start-feishubot/services/cardpool"
	"sync"
)

var (
	pool     *cardpool.CardPool
	poolOnce sync.Once
)

// InitCardPool 初始化卡片池
func InitCardPool(createCardFn cardpool.CreateCardFn) {
	poolOnce.Do(func() {
		log.Printf("Starting card pool initialization")
		pool = cardpool.NewCardPool(createCardFn)
		log.Printf("Card pool initialized with size: %d", pool.GetPoolSize())
	})
}

// GetCardPool 获取卡片池实例
func GetCardPool() *cardpool.CardPool {
	return pool
}

// ShutdownCardPool 关闭卡片池
func ShutdownCardPool() {
	if pool != nil {
		pool.Stop()
		log.Printf("Card pool shutdown")
	}
}
