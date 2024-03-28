package eventserver

import (
	"container/list"
	"context"
	"sync"
	"time"

	"errors"

	"github.com/ceres919/simple-grpc/internal/models"
	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	uid "github.com/hitoshi44/go-uid64"
)

type Server struct {
	eventmanager.UnimplementedEventsServer
	eventsMap           map[int64]map[int64]*list.Element
	eventsList          *list.List
	eventsChannel       chan *models.Event
	listChangingChannel chan bool
	mut                 sync.RWMutex
}

func NewServerEvent(events ...*eventmanager.MakeEventRequest) *Server {
	publisherChan := make(chan *models.Event, 10000)
	publish(publisherChan)
	serv := Server{
		eventsMap:           make(map[int64]map[int64]*list.Element),
		eventsList:          list.New(),
		eventsChannel:       publisherChan,
		listChangingChannel: make(chan bool, 1),
	}

	// TODO добавить дополнительный массив ивентов для тестирования?
	for _, event := range events {
		serv.MakeEvent(context.Background(), event)
	}
	go serv.timerQueue()
	return &serv
}

func (s *Server) passedEvents(currentTime time.Time) {
	for e := s.eventsList.Front(); e != nil; {
		event := e.Value.(*models.Event)
		eTime := time.UnixMilli(event.Time).UTC()
		timeDuration := eTime.Sub(currentTime)
		if timeDuration <= 0 {
			s.eventsChannel <- event
			s.mut.Lock()
			next := e.Next()
			delete(s.eventsMap[event.SenderId], event.EventId)
			s.eventsList.Remove(e)
			e = next
			s.mut.Unlock()
			continue
		}
		return
	}
}

func (s *Server) timerQueue() {
	for {

		if s.eventsList.Len() == 0 {
			<-s.listChangingChannel
			continue
		}

		eventPtr := s.eventsList.Front()
		event := eventPtr.Value.(*models.Event)
		t1 := time.Now().UTC()
		t2 := time.UnixMilli(event.Time).UTC()
		timeDuration := t2.Sub(t1)

		if timeDuration <= 0 {
			s.passedEvents(t1)
			continue
		}
		timer := time.NewTimer(timeDuration)

		select {
		case <-timer.C:
			s.eventsChannel <- event
			s.mut.Lock()
			delete(s.eventsMap[event.SenderId], event.EventId)
			s.eventsList.Remove(eventPtr)
			s.mut.Unlock()

		case <-s.listChangingChannel:
			timer.Stop()

		}
	}
}

func (s *Server) MakeEvent(ctx context.Context, req *eventmanager.MakeEventRequest) (*eventmanager.EventIdResponse, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	g, _ := uid.NewGenerator(0)
	event_id, _ := g.Gen()
	event := &models.Event{SenderId: req.SenderId, EventId: event_id.ToInt(), Time: req.Time, Name: req.Name}
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
			item := e.Value.(*models.Event)
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
	return &eventmanager.EventIdResponse{
		EventId: event_id.ToInt(),
	}, nil
}

func (s *Server) GetEvent(ctx context.Context, req *eventmanager.GetEventRequest) (*eventmanager.EventResponse, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	_, exist := s.eventsMap[req.SenderId]
	if exist {
		eventPtr, existence := s.eventsMap[req.SenderId][req.EventId]
		if existence {
			event := eventPtr.Value.(*models.Event)
			return &eventmanager.EventResponse{
				SenderId: event.SenderId,
				EventId:  event.EventId,
				Time:     event.Time,
				Name:     event.Name,
			}, nil
		} else {
			return nil, errors.New("not found")
		}
	} else {
		return nil, errors.New("not found")
	}
}

func (s *Server) GetEvents(req *eventmanager.GetEventsRequest, stream eventmanager.Events_GetEventsServer) error {
	s.mut.Lock()
	defer s.mut.Unlock()
	found := false
	senderID := req.SenderId
	_, exist := s.eventsMap[senderID]
	if exist {
		for _, eventPtr := range s.eventsMap[senderID] {
			event := eventPtr.Value.(*models.Event)
			if req.FromTime <= event.Time && event.Time <= req.ToTime {
				found = true
				if err := stream.Send(ArchiveEvent(*event)); err != nil {
					return err
				}
			}
		}
		if !found {
			return errors.New("not found")
		}
	} else {
		return errors.New("not found senders events")
	}
	return nil
}

func (s *Server) DeleteEvent(
	ctx context.Context,
	req *eventmanager.DeleteEventRequest,
) (*eventmanager.EventIdResponse, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	_, existence := s.eventsMap[req.SenderId]
	if !existence {
		return nil, errors.New("not found")
	}
	eventPtr, existence := s.eventsMap[req.SenderId][req.EventId]
	if existence {
		delete(s.eventsMap[req.SenderId], req.EventId)
		frontItem := s.eventsList.Front().Value
		s.eventsList.Remove(eventPtr)
		if eventPtr.Value == frontItem {
			s.listChangingChannel <- true
		}
		return &eventmanager.EventIdResponse{
			EventId: req.EventId,
		}, nil
	}
	return nil, errors.New("not found")
}

func (s *Server) CheckSenderEventExistence(sid int64, eid int64) bool {
	s.mut.Lock()
	defer s.mut.Unlock()
	_, exist := s.eventsMap[sid]
	if !exist {
		return false
	}
	_, exist = s.eventsMap[sid][eid]
	return exist
}

func ArchiveEvent(event models.Event) *eventmanager.EventResponse {
	return &eventmanager.EventResponse{
		SenderId: event.SenderId,
		EventId:  event.EventId,
		Time:     event.Time,
		Name:     event.Name,
	}
}
