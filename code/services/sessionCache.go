package services

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"sort"
	"start-feishubot/services/ai"
	"start-feishubot/services/openai"
	"sync"
	"sync/atomic"
	"time"

	"github.com/patrickmn/go-cache"
)

type SessionMode string

const (
	ModePicCreate SessionMode = "pic_create"
	ModePicVary   SessionMode = "pic_vary"
	ModeGPT       SessionMode = "gpt"
)

// 缓存配置常量
const (
	DefaultExpiration = 12 * time.Hour  // 默认过期时间
	CleanupInterval   = 1 * time.Hour   // 清理间隔
	MaxSessionsPerUser = 10             // 每个用户最大会话数
	MaxTotalSessions  = 10000          // 总会话数限制
	MaxMessageLength  = 4096           // 单条消息最大长度
	MaxMessagesPerSession = 100        // 每个会话最大消息数
	MemoryLimit       = int64(4 * 1024 * 1024 * 1024) // 4GB内存限制，总内存6GB
)

// 内存阈值常量
const (
	MemoryThresholdCleanup = MemoryLimit * 9 / 10 // 90%触发清理
	MemoryThresholdWarn    = MemoryLimit * 8 / 10 // 80%触发警告
)

// SessionMeta 会话元数据
type SessionMeta struct {
	Mode       SessionMode  `json:"mode"`
	Messages   []ai.Message `json:"messages,omitempty"`
	UserId     string      `json:"user_id"`     
	UpdatedAt  time.Time   `json:"updated_at"`  
	MessageNum int         `json:"message_num"` 
	Size       int64       `json:"size"`        // 会话大小（字节）
	PicResolution string    `json:"pic_resolution,omitempty"` // 图片分辨率设置
	SystemMsg []openai.Messages `json:"system_msg,omitempty"` // 系统消息
	CardId     string      `json:"card_id,omitempty"`     // 卡片ID
	MessageId  string      `json:"message_id,omitempty"`  // 消息ID
	ConversationID string  `json:"conversation_id,omitempty"` // Dify对话ID
	CacheAddress string    `json:"cache_address,omitempty"`   // 消息缓存地址
}

// SessionService 会话服务
type SessionService struct {
	cache *cache.Cache
	mu    sync.RWMutex 
	
	// 统计信息
	totalSessions   int32          // 总会话数
	totalMemoryUsed int64          // 总内存使用
	userSessionCount map[string]int // 用户会话计数
	stats           *SessionStats   // 会话统计

	// 新增: 用户消息索引
	userMessageIndex map[string]map[string]*SessionMeta // map[userId]map[messageId]*SessionMeta
}

// SessionStats 会话统计
type SessionStats struct {
	TotalSessions      int32     `json:"total_sessions"`
	TotalMemoryUsedMB  float64   `json:"total_memory_used_mb"`
	ActiveUsers        int       `json:"active_users"`
	AvgSessionSize     float64   `json:"avg_session_size"`
	LastCleanupTime    time.Time `json:"last_cleanup_time"`
	CleanedSessions    int       `json:"cleaned_sessions"`
}

// SessionServiceCacheInterface 会话服务接口
type SessionServiceCacheInterface interface {
	GetMessages(sessionId string) []ai.Message
	SetMessages(sessionId string, userId string, messages []ai.Message, cardId string, messageId string, conversationID string, cacheAddress string) error
	GetMode(sessionId string) SessionMode
	SetMode(sessionId string, mode SessionMode)
	Clear(sessionId string)
	ClearUserSessions(userId string)
	GetUserSessions(userId string) []string
	CleanExpiredSessions() int
	GetStats() SessionStats
	SetPicResolution(sessionId string, resolution string)
	GetPicResolution(sessionId string) string
	SetMsg(sessionId string, msg []openai.Messages)
	GetSessionMeta(sessionId string) (*SessionMeta, bool)
	IsDuplicateMessage(userId string, messageId string) bool
	GetCardID(sessionId string, userId string, messageId string) (string, error)
}

var (
	sessionServices *SessionService
	once           sync.Once
)

// GetSessionCache 获取会话缓存单例
func GetSessionCache() SessionServiceCacheInterface {
	once.Do(func() {
		sessionServices = &SessionService{
			cache:            cache.New(DefaultExpiration, CleanupInterval),
			userSessionCount: make(map[string]int),
			stats:           &SessionStats{},
			userMessageIndex: make(map[string]map[string]*SessionMeta),
		}
		
		// 启动定期清理
		go sessionServices.periodicCleanup()
		
		// 启动内存监控
		go sessionServices.monitorMemory()
	})
	return sessionServices
}

// GetMode 获取会话模式
func (s *SessionService) GetMode(sessionId string) SessionMode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		return ModeGPT
	}
	sessionMeta := sessionContext.(*SessionMeta)
	return sessionMeta.Mode
}

// SetMode 设置会话模式
func (s *SessionService) SetMode(sessionId string, mode SessionMode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		sessionMeta := &SessionMeta{
			Mode:      mode,
			UpdatedAt: time.Now(),
		}
		s.cache.Set(sessionId, sessionMeta, DefaultExpiration)
		return
	}
	sessionMeta := sessionContext.(*SessionMeta)
	sessionMeta.Mode = mode
	sessionMeta.UpdatedAt = time.Now()
	s.cache.Set(sessionId, sessionMeta, DefaultExpiration)
}

// GetMessages 获取会话消息
func (s *SessionService) GetMessages(sessionId string) []ai.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		return nil
	}
	sessionMeta := sessionContext.(*SessionMeta)
	
	// 复制消息并添加session_id到元数据
	messages := make([]ai.Message, len(sessionMeta.Messages))
	for i, msg := range sessionMeta.Messages {
		messages[i] = msg
		// 确保元数据存在
		if messages[i].Metadata == nil {
			messages[i].Metadata = make(map[string]string)
		}
		// 添加session_id到元数据
		messages[i].Metadata["session_id"] = sessionId
	}
	
	return messages
}

// SetMessages 设置会话消息
func (s *SessionService) SetMessages(sessionId string, userId string, messages []ai.Message, cardId string, messageId string, conversationID string, cacheAddress string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否为重复消息
	if s.isDuplicateMessageUnsafe(userId, messageId) {
		return fmt.Errorf("duplicate message")
	}

	// 验证消息
	for _, msg := range messages {
		if err := msg.Validate(); err != nil {
			return fmt.Errorf("invalid message: %v", err)
		}
		if len(msg.Content) > MaxMessageLength {
			return fmt.Errorf("message too long: %d > %d", len(msg.Content), MaxMessageLength)
		}
	}

	if len(messages) > MaxMessagesPerSession {
		return fmt.Errorf("too many messages: %d > %d", len(messages), MaxMessagesPerSession)
	}

	// 检查用户会话数限制
	if s.userSessionCount[userId] >= MaxSessionsPerUser {
		// 清理该用户最旧的会话
		s.cleanOldestUserSession(userId)
	}

	// 计算会话大小
	size := s.calculateSessionSize(messages)

	// 检查内存限制
	if atomic.LoadInt64(&s.totalMemoryUsed)+size > MemoryLimit {
		// 触发清理
		s.forceCleanup()
		// 再次检查
		if atomic.LoadInt64(&s.totalMemoryUsed)+size > MemoryLimit {
			return fmt.Errorf("memory limit exceeded")
		}
	}

	sessionContext, exists := s.cache.Get(sessionId)
	var sessionMeta *SessionMeta
	if !exists {
		// 检查总会话数限制
		if atomic.LoadInt32(&s.totalSessions) >= int32(MaxTotalSessions) {
			s.forceCleanup()
			if atomic.LoadInt32(&s.totalSessions) >= int32(MaxTotalSessions) {
				return fmt.Errorf("max sessions limit exceeded")
			}
		}
		
		sessionMeta = &SessionMeta{
			Messages:       messages,
			UserId:         userId,
			UpdatedAt:      time.Now(),
			MessageNum:     len(messages),
			Size:           size,
			CardId:         cardId,
			MessageId:      messageId,
			ConversationID: conversationID,
			CacheAddress:   cacheAddress,
		}
		atomic.AddInt32(&s.totalSessions, 1)
		s.userSessionCount[userId]++
	} else {
		sessionMeta = sessionContext.(*SessionMeta)
		atomic.AddInt64(&s.totalMemoryUsed, -sessionMeta.Size) // 减去旧大小
		sessionMeta.Messages = messages
		sessionMeta.UpdatedAt = time.Now()
		sessionMeta.MessageNum = len(messages)
		sessionMeta.Size = size
		sessionMeta.CardId = cardId
		sessionMeta.MessageId = messageId
		sessionMeta.ConversationID = conversationID
		sessionMeta.CacheAddress = cacheAddress
	}

	atomic.AddInt64(&s.totalMemoryUsed, size)
	s.cache.Set(sessionId, sessionMeta, DefaultExpiration)

	// 更新用户消息索引
	if _, ok := s.userMessageIndex[userId]; !ok {
		s.userMessageIndex[userId] = make(map[string]*SessionMeta)
	}
	s.userMessageIndex[userId][messageId] = sessionMeta

	return nil
}

// Clear 清除会话
func (s *SessionService) Clear(sessionId string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item, exists := s.cache.Get(sessionId); exists {
		meta := item.(*SessionMeta)
		atomic.AddInt64(&s.totalMemoryUsed, -meta.Size)
		atomic.AddInt32(&s.totalSessions, -1)
		s.userSessionCount[meta.UserId]--
		if s.userSessionCount[meta.UserId] <= 0 {
			delete(s.userSessionCount, meta.UserId)
		}

		// 从用户消息索引中删除
		if userMessages, ok := s.userMessageIndex[meta.UserId]; ok {
			delete(userMessages, meta.MessageId)
			if len(userMessages) == 0 {
				delete(s.userMessageIndex, meta.UserId)
			}
		}
	}
	s.cache.Delete(sessionId)
}

// ClearUserSessions 清除用户所有会话
func (s *SessionService) ClearUserSessions(userId string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := s.cache.Items()
	for sessionId, item := range items {
		if meta, ok := item.Object.(*SessionMeta); ok && meta.UserId == userId {
			atomic.AddInt64(&s.totalMemoryUsed, -meta.Size)
			atomic.AddInt32(&s.totalSessions, -1)
			s.cache.Delete(sessionId)
		}
	}
	delete(s.userSessionCount, userId)
}

// GetUserSessions 获取用户所有会话ID
func (s *SessionService) GetUserSessions(userId string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []string
	items := s.cache.Items()
	for sessionId, item := range items {
		if meta, ok := item.Object.(*SessionMeta); ok && meta.UserId == userId {
			sessions = append(sessions, sessionId)
		}
	}
	return sessions
}

// CleanExpiredSessions 清理过期会话
func (s *SessionService) CleanExpiredSessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	expiredTime := time.Now().Add(-DefaultExpiration)
	items := s.cache.Items()
	for sessionId, item := range items {
		if meta, ok := item.Object.(*SessionMeta); ok {
			if meta.UpdatedAt.Before(expiredTime) {
				atomic.AddInt64(&s.totalMemoryUsed, -meta.Size)
				atomic.AddInt32(&s.totalSessions, -1)
				s.userSessionCount[meta.UserId]--
				if s.userSessionCount[meta.UserId] <= 0 {
					delete(s.userSessionCount, meta.UserId)
				}
				s.cache.Delete(sessionId)
				count++
			}
		}
	}
	
	s.stats.LastCleanupTime = time.Now()
	s.stats.CleanedSessions += count
	return count
}

// GetStats 获取统计信息
func (s *SessionService) GetStats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.stats.TotalSessions = atomic.LoadInt32(&s.totalSessions)
	s.stats.TotalMemoryUsedMB = float64(atomic.LoadInt64(&s.totalMemoryUsed)) / 1024 / 1024
	s.stats.ActiveUsers = len(s.userSessionCount)
	if s.stats.TotalSessions > 0 {
		s.stats.AvgSessionSize = float64(s.totalMemoryUsed) / float64(s.totalSessions)
	}
	return *s.stats
}

// 内部方法

func (s *SessionService) calculateSessionSize(messages []ai.Message) int64 {
	bytes, _ := json.Marshal(messages)
	return int64(len(bytes))
}

func (s *SessionService) cleanOldestUserSession(userId string) {
	var oldestSession string
	var oldestTime time.Time
	items := s.cache.Items()
	for sessionId, item := range items {
		if meta, ok := item.Object.(*SessionMeta); ok && meta.UserId == userId {
			if oldestSession == "" || meta.UpdatedAt.Before(oldestTime) {
				oldestSession = sessionId
				oldestTime = meta.UpdatedAt
			}
		}
	}
	if oldestSession != "" {
		s.Clear(oldestSession)
	}
}

func (s *SessionService) forceCleanup() {
	// 首先清理过期会话
	s.CleanExpiredSessions()
	
	// 如果还需要清理，按最后访问时间清理
	if atomic.LoadInt64(&s.totalMemoryUsed) > MemoryThresholdCleanup {
		items := s.cache.Items()
		sessions := make([]*struct {
			id   string
			meta *SessionMeta
		}, 0, len(items))
		
		for id, item := range items {
			if meta, ok := item.Object.(*SessionMeta); ok {
				sessions = append(sessions, &struct {
					id   string
					meta *SessionMeta
				}{id, meta})
			}
		}
		
		// 按最后访问时间排序
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].meta.UpdatedAt.Before(sessions[j].meta.UpdatedAt)
		})
		
		// 清理最旧的20%会话
		cleanCount := len(sessions) / 5
		for i := 0; i < cleanCount; i++ {
			s.Clear(sessions[i].id)
		}
	}
}

func (s *SessionService) periodicCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	for range ticker.C {
		s.CleanExpiredSessions()
	}
}

// SetMsg 设置系统消息
func (s *SessionService) SetMsg(sessionId string, msg []openai.Messages) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		sessionMeta := &SessionMeta{
			UpdatedAt: time.Now(),
			SystemMsg: msg,
		}
		s.cache.Set(sessionId, sessionMeta, DefaultExpiration)
		return
	}
	sessionMeta := sessionContext.(*SessionMeta)
	sessionMeta.UpdatedAt = time.Now()
	sessionMeta.SystemMsg = msg
	s.cache.Set(sessionId, sessionMeta, DefaultExpiration)
}

func (s *SessionService) monitorMemory() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		// 如果总内存使用超过限制的80%，触发清理
		if uint64(m.Alloc) > uint64(MemoryThresholdWarn) {
			log.Printf("Memory usage high (%.2f MB), triggering cleanup", float64(m.Alloc)/1024/1024)
			s.forceCleanup()
		}
	}
}

// SetPicResolution 设置图片分辨率
func (s *SessionService) SetPicResolution(sessionId string, resolution string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		sessionMeta := &SessionMeta{
			UpdatedAt:     time.Now(),
			PicResolution: resolution,
		}
		s.cache.Set(sessionId, sessionMeta, DefaultExpiration)
		return
	}
	sessionMeta := sessionContext.(*SessionMeta)
	sessionMeta.PicResolution = resolution
	sessionMeta.UpdatedAt = time.Now()
	s.cache.Set(sessionId, sessionMeta, DefaultExpiration)
}

// GetPicResolution 获取图片分辨率
func (s *SessionService) GetPicResolution(sessionId string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		return "512x512" // 默认分辨率
	}
	sessionMeta := sessionContext.(*SessionMeta)
	if sessionMeta.PicResolution == "" {
		return "512x512" // 默认分辨率
	}
	return sessionMeta.PicResolution
}

// GetSessionMeta 获取会话元数据
func (s *SessionService) GetSessionMeta(sessionId string) (*SessionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionContext, ok := s.cache.Get(sessionId)
	if !ok {
		return nil, false
	}
	sessionMeta := sessionContext.(*SessionMeta)
	return sessionMeta, true
}

// IsDuplicateMessage 检查是否为重复消息
func (s *SessionService) IsDuplicateMessage(userId string, messageId string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isDuplicateMessageUnsafe(userId, messageId)
}

// isDuplicateMessageUnsafe 内部使用的非线程安全版本
func (s *SessionService) isDuplicateMessageUnsafe(userId string, messageId string) bool {
	if userMessages, ok := s.userMessageIndex[userId]; ok {
		_, exists := userMessages[messageId]
		return exists
	}
	return false
}

// GetSessionInfo 获取会话信息
func (s *SessionService) GetSessionInfo(userId string, messageId string) (*SessionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 从用户消息索引中获取会话信息
	if userMessages, ok := s.userMessageIndex[userId]; ok {
		if sessionMeta, exists := userMessages[messageId]; exists {
			return sessionMeta, nil
		}
	}

	// 如果在用户消息索引中找不到，遍历所有会话查找
	items := s.cache.Items()
	for _, item := range items {
		if sessionMeta, ok := item.Object.(*SessionMeta); ok {
			if sessionMeta.UserId == userId && sessionMeta.MessageId == messageId {
				return sessionMeta, nil
			}
		}
	}

	return nil, fmt.Errorf("session info not found for the given user and message")
}
