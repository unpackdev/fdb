package rpc

import "github.com/google/uuid"

func generateSubscriptionID() string {
	return uuid.New().String()
}
