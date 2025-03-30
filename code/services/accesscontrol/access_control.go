package accesscontrol

import (
	"start-feishubot/config"
	"start-feishubot/utils"
	"sync"
)

// InitAccessControl initializes the access control system
func InitAccessControl(cfg *config.Config) error {
	InitConfig(cfg)
	// Initialize the access control system with the provided configuration
	// For now, we'll just set the current date flag
	currentDateFlag = utils.GetCurrentDateAsString()
	return nil
}

var accessCountMap = sync.Map{}
var currentDateFlag = ""

/*
CheckAllowAccessThenIncrement If user has accessed more than 100 times according to accessCountMap, return false.
Otherwise, return true and increase the access count by 1
*/
func CheckAllowAccessThenIncrement(userId *string) bool {

	// Begin a new day, clear the accessCountMap
	currentDateAsString := utils.GetCurrentDateAsString()
	if currentDateFlag != currentDateAsString {
		accessCountMap = sync.Map{}
		currentDateFlag = currentDateAsString
	}

	if CheckAllowAccess(userId) {
		accessedCount, ok := accessCountMap.Load(*userId)
		if !ok {
			accessCountMap.Store(*userId, 1)
		} else {
			accessCountMap.Store(*userId, accessedCount.(int)+1)
		}
		return true
	} else {
		return false
	}
}

func CheckAllowAccess(userId *string) bool {

	if GetConfig().AccessControlMaxCountPerUserPerDay <= 0 {
		return true
	}

	accessedCount, ok := accessCountMap.Load(*userId)

	if !ok {
		accessCountMap.Store(*userId, 0)
		return true
	}

	// If the user has accessed more than 100 times, return false
	if accessedCount.(int) >= GetConfig().AccessControlMaxCountPerUserPerDay {
		return false
	}

	// Otherwise, return true
	return true
}

func GetCurrentDateFlag() string {
	return currentDateFlag
}

func GetAccessCountMap() *sync.Map {
	return &accessCountMap
}
