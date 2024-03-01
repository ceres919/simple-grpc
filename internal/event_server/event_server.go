package eventserver

import (
	"container/list"
	"context"
	"time"

	"github.com/ceres919/simple-grpc/internal/models"
	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	uid "github.com/hitoshi44/go-uid64"
)

type Server struct {
	eventmanager.UnimplementedEventsServer
	eventsMap           map[int64]map[int64]*list.Element
	eventsList          *list.List
	eventsChannel       chan models.Event
	listChangingChannel chan bool
}

func NewServerEvent() *Server {
	publisherChan := make(chan models.Event, 1)
	publish(publisherChan)
	serv := Server{
		eventsMap:           make(map[int64]map[int64]*list.Element),
		eventsList:          list.New(),
		eventsChannel:       publisherChan,
		listChangingChannel: make(chan bool, 1),
	}
	go serv.timerQueue()
	return &serv
}

func (s *Server) timerQueue() {
	for {
		if s.eventsList.Len() == 0 {
			<-s.listChangingChannel
			continue
		}
		eventPtr := s.eventsList.Front()
		event := eventPtr.Value.(models.Event)
		t1 := time.Now().UTC()
		t2 := time.UnixMilli(event.Time).UTC()
		timeDuration := t2.Sub(t1)
		timer := time.NewTimer(timeDuration)

		select {

		case <-timer.C:
			s.eventsChannel <- event
			delete(s.eventsMap[event.SenderId], event.EventId)
			s.eventsList.Remove(eventPtr)

		case <-s.listChangingChannel:
			timer.Stop()

		}
	}
}

func (s *Server) MakeEvent(ctx context.Context, req *eventmanager.MakeEventRequest) (*eventmanager.MakeEventResponse, error) {
	g, _ := uid.NewGenerator(0)
	event_id, _ := g.Gen()

	//TODO сравнить что быстрее при больших объемах данных
	//event := &models.Event{SenderId: req.SenderId, EventId: event_id.ToInt(), Time: req.Time, Name: req.Name}
	event := models.Event{SenderId: req.SenderId, EventId: event_id.ToInt(), Time: req.Time, Name: req.Name}

	var eventPtr *list.Element
	_, existence := s.eventsMap[req.SenderId]

	if !existence {
		s.eventsMap[req.SenderId] = make(map[int64]*list.Element)
	}
	if s.eventsList.Len() == 0 {
		eventPtr = s.eventsList.PushBack(event)
		s.listChangingChannel <- true
	} else {
		for e := s.eventsList.Back(); e != nil; e = e.Prev() {
			item := e.Value.(models.Event)
			if event.Time >= item.Time {
				eventPtr = s.eventsList.InsertAfter(event, e)
				break
			} else if e == s.eventsList.Front() && item.Time > event.Time {
				eventPtr = s.eventsList.InsertBefore(event, e)
				s.listChangingChannel <- true
				break
			}
		}
	}
	s.eventsMap[event.SenderId][event.EventId] = eventPtr
	return &eventmanager.MakeEventResponse{
		EventId: event_id.ToInt(),
	}, nil
}

func (s *Server) GetEvent(ctx context.Context, req *eventmanager.GetEventRequest) (*eventmanager.GetEventResponse, error) {
	eventPtr, existence := s.eventsMap[req.SenderId][req.EventId]
	event := eventPtr.Value.(models.Event)
	if existence {
		return &eventmanager.GetEventResponse{
			SenderId: event.SenderId,
			EventId:  event.EventId,
			Time:     event.Time,
			Name:     event.Name,
		}, nil
	}
	return nil, nil
}

func (s *Server) GetEvents(req *eventmanager.GetEventsRequest, stream eventmanager.Events_GetEventsServer) error {
	senderID := req.SenderId
	for _, eventsByClient := range s.eventsMap {
		for _, eventPtr := range eventsByClient {
			event := eventPtr.Value.(models.Event)
			if event.SenderId == senderID {
				if req.FromTime < event.Time && event.Time < req.ToTime {
					if err := stream.Send(ArchiveEvent(event)); err != nil {
						return err
					}
				} else {
					return nil
				}
			} else {
				return nil
			}
		}
	}
	return nil
}

func (s *Server) DeleteEvent(ctx context.Context, req *eventmanager.DeleteEventRequest) (*eventmanager.DeleteEventResponse, error) {
	eventPtr, existence := s.eventsMap[req.SenderId][req.EventId]
	if existence {
		delete(s.eventsMap[req.SenderId], req.EventId)
		frontItem := s.eventsList.Front().Value
		s.eventsList.Remove(eventPtr)
		if eventPtr.Value == frontItem {
			s.listChangingChannel <- true
		}
		return &eventmanager.DeleteEventResponse{
			EventId: req.EventId,
		}, nil
	}
	return nil, nil
}

func ArchiveEvent(event models.Event) *eventmanager.GetEventsResponse {
	return &eventmanager.GetEventsResponse{
		SenderId: event.SenderId,
		EventId:  event.EventId,
		Time:     event.Time,
		Name:     event.Name,
	}
}
