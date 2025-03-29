// Add this new function at the end of the file
func updateTextCard(ctx context.Context, content string, cardInfo *CardInfo) error {
	log.Printf("Starting updateTextCard for card ID: %s", cardInfo.CardId)

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := cardService.UpdateCard(ctx, cardInfo.CardId, content)
		if err == nil {
			log.Printf("Card update successful for card ID: %s", cardInfo.CardId)
			return nil
		}

		log.Printf("Attempt %d failed to update card ID %s: %v", i+1, cardInfo.CardId, err)

		if i < maxRetries-1 {
			// Wait for a short duration before retrying
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while retrying card update: %w", ctx.Err())
			case <-time.After(time.Duration(i+1) * 100 * time.Millisecond):
				// Exponential backoff
			}
		}
	}

	return fmt.Errorf("failed to update card after %d attempts", maxRetries)
}
