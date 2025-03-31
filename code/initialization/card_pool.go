package initialization

import (
	"log"
	"start-feishubot/services/cardpool"
	"sync"
)

var (
	cardPoolInstance *cardpool.CardPool
	cardPoolOnce     sync.Once
)

// InitCardPool 初始化卡片池
func InitCardPool(createCardFn cardpool.CreateCardFn) error {
	cardPoolOnce.Do(func() {
		log.Printf("Initializing card pool")
		cardPoolInstance = &cardpool.CardPool{}
		cardPoolInstance.Init(createCardFn)
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
