package models

import "github.com/google/uuid"

type Event struct {
	SenderId int64
	EventId  uuid.UUID
	Time     int64
	Name     string
}
