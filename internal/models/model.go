package models

type Event struct {
	SenderId int64
	EventId  int64
	Time     int64
	Name     string
}
