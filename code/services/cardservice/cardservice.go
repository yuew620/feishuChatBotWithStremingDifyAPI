package cardservice

import (
	"context"
	"log"
	"start-feishubot/handlers"
	"start-feishubot/services/cardpool"
	"sync"
)

var (
	pool     *cardpool.CardPool
	poolOnce sync.Once
)

// InitCardPool 初始化卡片池
func InitCardPool() {
	poolOnce.Do(func() {
		// 创建卡片的函数
		createCardFn := func(ctx context.Context) (string, error) {
			content := "正在思考中，请稍等..."
			cardID, err := handlers.CreateCardEntity(ctx, content)
			if err != nil {
				log.Printf("Failed to create card entity: %v", err)
				return "", err
			}
			return cardID, nil
		}

		// 创建卡片池
		pool = cardpool.NewCardPool(createCardFn)
		log.Printf("Card pool initialized")
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
