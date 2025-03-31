package initialization

import (
	"log"
	"start-feishubot/services/cardpool"
	"sync"
	"time"
)

var (
	cardPoolInstance *cardpool.CardPool
	cardPoolOnce     sync.Once
)

// InitCardPool 初始化卡片池
func InitCardPool(createCardFn cardpool.CreateCardFn) error {
	cardPoolOnce.Do(func() {
		log.Printf("[CardPool Init] ===== Starting card pool initialization =====")
		startTime := time.Now()
		
		log.Printf("[CardPool Init] Creating new card pool instance")
		cardPoolInstance = cardpool.NewCardPool(createCardFn)
		
		log.Printf("[CardPool Init] ===== Card pool initialization completed in %v, size: %d =====", 
			time.Since(startTime), 
			cardPoolInstance.GetPoolSize())
	})
	return nil
}

// ShutdownCardPool 关闭卡片池
func ShutdownCardPool() {
	if cardPoolInstance != nil {
		cardPoolInstance.Stop()
		cardPoolInstance = nil
	}
}
