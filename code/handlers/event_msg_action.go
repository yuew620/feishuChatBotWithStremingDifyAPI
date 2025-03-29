func sendOnProcess(a *ActionInfo, aiMessages []ai.Message) (*CardInfo, chan string, error) {
	log.Printf("Starting sendOnProcess for session %s", *a.info.sessionId)
	
	// 创建响应通道
	responseStream := make(chan string, 10)
	
	// 创建Dify消息处理函数
	difyHandler := func(ctx context.Context) error {
		log.Printf("Starting Dify handler for session %s", *a.info.sessionId)
		
		// 预处理消息，准备发送到Dify
		difyMessages := make([]dify.Messages, len(aiMessages))
		for i, msg := range aiMessages {
			difyMessages[i] = dify.Messages{
				Role:     msg.Role,
				Content:  msg.Content,
				Metadata: msg.Metadata,
			}
		}
		
		// 发送请求到Dify服务
		difyClient := initialization.GetDifyClient()
		log.Printf("Sending StreamChat request to Dify for session %s", *a.info.sessionId)
		
		streamStartTime := time.Now()
		
		// 创建一个带有超时的上下文
		streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		
		errChan := make(chan error, 1)
		go func() {
			errChan <- difyClient.StreamChat(streamCtx, difyMessages, responseStream)
		}()
		
		select {
		case err := <-errChan:
			streamDuration := time.Since(streamStartTime)
			if err != nil {
				log.Printf("Error in Dify StreamChat for session %s: %v (duration: %v)", *a.info.sessionId, err, streamDuration)
				return fmt.Errorf("failed to send message to Dify: %w", err)
			}
			log.Printf("Dify StreamChat completed successfully for session %s (duration: %v)", *a.info.sessionId, streamDuration)
			return nil
		case <-streamCtx.Done():
			streamDuration := time.Since(streamStartTime)
			log.Printf("Dify StreamChat timed out for session %s after %v", *a.info.sessionId, streamDuration)
			return fmt.Errorf("Dify StreamChat timed out after %v", streamDuration)
		}
	}
	
	// 使用并行处理函数
	log.Printf("Calling sendOnProcessCardAndDify for session %s", *a.info.sessionId)
	cardInfo, err := sendOnProcessCardAndDify(*a.ctx, a.info.sessionId, a.info.msgId, difyHandler)
	if err != nil {
		log.Printf("Error in sendOnProcessCardAndDify for session %s: %v", *a.info.sessionId, err)
		return nil, nil, fmt.Errorf("failed to send processing card: %w", err)
	}
	
	log.Printf("Processing card sent successfully for session %s, card ID: %s", *a.info.sessionId, cardInfo.CardId)

	// 创建一个新的通道来处理和记录从Dify接收到的消息
	processedStream := make(chan string, 10)
	go func() {
		defer close(processedStream)
		lastMessageTime := time.Now()
		for msg := range responseStream {
			currentTime := time.Now()
			timeSinceLastMessage := currentTime.Sub(lastMessageTime)
			log.Printf("Received message from Dify for session %s: %s (time since last message: %v)", *a.info.sessionId, msg, timeSinceLastMessage)
			processedStream <- msg
			lastMessageTime = currentTime
		}
		log.Printf("Dify response stream closed for session %s", *a.info.sessionId)
	}()

	log.Printf("sendOnProcess completed for session %s", *a.info.sessionId)
	return cardInfo, processedStream, nil
}
