package eventserver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ceres919/simple-grpc/internal/models"
	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestServer_MakeEvent(t *testing.T) {
	currentTime := time.Now().AddDate(0, 0, 1).UnixMilli()
	s := NewServerEvent()

	type test struct {
		name    string
		req     *eventmanager.MakeEventRequest
		want    bool
		wantErr error
	}
	var tests []test
	for i := 0; i < 100; i++ {
		tests = append(tests, test{
			name: fmt.Sprintf("Test %d", i),
			req: &eventmanager.MakeEventRequest{
				SenderId: int64(i),
				Time:     currentTime,
				Name:     fmt.Sprintf("User %d", i),
			},
			want:    true,
			wantErr: nil,
		})
	}

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			defer wg.Done()
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.MakeEvent(context.Background(), tt.req)
				require.NoError(t, err)
				actual := s.CheckSenderEventExistence(tt.req.SenderId, got.EventId)
				require.Equal(t, actual, tt.want)
			})
		}(tt)
	}
	wg.Wait()
}

func mockEventMaker(s *Server, givenTime int64) (*map[int64]map[string][]byte, error) {
	eventsIdMap := make(map[int64]map[string][]byte)
	oneMonthLater := time.UnixMilli(givenTime).AddDate(0, 1, 0).UnixMilli()
	eventsData := []struct {
		SenderId int64
		Time     int64
		Name     string
	}{
		{SenderId: 1, Time: givenTime, Name: "User1"},
		{SenderId: 2, Time: givenTime, Name: "User2"},
		{SenderId: 2, Time: oneMonthLater, Name: "User2Again"},
		{SenderId: 3, Time: givenTime, Name: "User3"},
	}

	for _, eventData := range eventsData {
		e, err := s.MakeEvent(context.Background(), &eventmanager.MakeEventRequest{
			SenderId: eventData.SenderId,
			Time:     eventData.Time,
			Name:     eventData.Name,
		})

		_, existence := eventsIdMap[eventData.SenderId]
		if !existence {
			eventsIdMap[eventData.SenderId] = make(map[string][]byte)
		}
		eventsIdMap[eventData.SenderId][eventData.Name] = e.EventId
		if err != nil {
			return nil, err
		}
	}

	return &eventsIdMap, nil
}

func setupTest(t *testing.T, currentTime time.Time) (*Server, *map[int64]map[string][]byte) {
	s := NewServerEvent()
	var idMap *map[int64]map[string][]byte
	t.Run("Mocking events", func(t *testing.T) {
		var err error
		idMap, err = mockEventMaker(s, currentTime.UnixMilli())
		if err != nil {
			t.Error(err)
		}
	})
	t.Log("Events created")
	return s, idMap
}

func TestServer_GetEvent(t *testing.T) {
	oneMonthLaterT := time.Now().AddDate(0, 1, 0)
	s, idMapPtr := setupTest(t, oneMonthLaterT)
	idMap := *idMapPtr

	type test struct {
		name    string
		req     *eventmanager.GetEventRequest
		want    *eventmanager.EventResponse
		wantErr error
	}
	var tests []test

	tests = append(tests, test{
		name: "Test 1",
		req: &eventmanager.GetEventRequest{
			SenderId: 7,
			EventId:  []byte("78787-jgnj"),
		},
		want:    nil,
		wantErr: fmt.Errorf("not found"),
	}, test{
		name: "Test 2",
		req: &eventmanager.GetEventRequest{
			SenderId: 1,
			EventId:  idMap[1]["User1"],
		},
		want:    &eventmanager.EventResponse{},
		wantErr: nil,
	}, test{
		name: "Test 3",
		req: &eventmanager.GetEventRequest{
			SenderId: 2,
			EventId:  idMap[2]["User2"],
		},
		want:    &eventmanager.EventResponse{},
		wantErr: nil,
	}, test{
		name: "Test 4",
		req: &eventmanager.GetEventRequest{
			SenderId: 3,
			EventId:  idMap[3]["User3"],
		},
		want:    &eventmanager.EventResponse{},
		wantErr: nil,
	}, test{
		name: "Test 5",
		req: &eventmanager.GetEventRequest{
			SenderId: 3,
			EventId:  idMap[2]["User2"],
		},
		want:    nil,
		wantErr: fmt.Errorf("not found"),
	}, test{
		name: "Test 6",
		req: &eventmanager.GetEventRequest{
			SenderId: 1,
			EventId:  []byte("78787-jgnj"),
		},
		want:    nil,
		wantErr: fmt.Errorf("not found"),
	})

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			t.Run(tt.name, func(t *testing.T) {
				defer wg.Done()
				got, err := s.GetEvent(context.Background(), tt.req)
				if tt.wantErr == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, tt.wantErr.Error())
				}
				if tt.want == nil {
					require.Nil(t, got)
				} else {
					require.Equal(t, got.EventId, tt.req.EventId)
				}
			})
		}(tt)
	}
	wg.Wait()
}

type streamMock struct {
	grpc.ServerStream
	sent []*eventmanager.EventResponse
}

func makeStreamMock() *streamMock {
	return &streamMock{
		sent: []*eventmanager.EventResponse{},
	}
}
func (st *streamMock) Send(resp *eventmanager.EventResponse) error {
	st.sent = append(st.sent, resp)
	return nil
}

func TestServer_GetEvents(t *testing.T) {
	oneMonthLaterT := time.Now().AddDate(0, 1, 0)
	s, idMapPtr := setupTest(t, oneMonthLaterT)
	idMap := *idMapPtr
	type test struct {
		name    string
		req     *eventmanager.GetEventsRequest
		want    []*eventmanager.EventResponse
		wantErr error
	}
	var tests []test

	tests = append(tests, test{
		name: "Test 1",
		req: &eventmanager.GetEventsRequest{
			SenderId: 2,
			FromTime: oneMonthLaterT.UnixMilli(),
			ToTime:   oneMonthLaterT.AddDate(0, 2, 0).UnixMilli(),
		},
		want: []*eventmanager.EventResponse{
			{
				SenderId: 2,
				EventId:  idMap[2]["User2"],
				Time:     oneMonthLaterT.UnixMilli(),
				Name:     "User2",
			},
			{
				SenderId: 2,
				EventId:  idMap[2]["User2Again"],
				Time:     oneMonthLaterT.AddDate(0, 1, 0).UnixMilli(),
				Name:     "User2Again",
			},
		},
		wantErr: nil,
	}, test{
		name: "Test 2",
		req: &eventmanager.GetEventsRequest{
			SenderId: 2,
			FromTime: oneMonthLaterT.UnixMilli(),
			ToTime:   oneMonthLaterT.AddDate(0, 0, 2).UnixMilli(),
		},
		want: []*eventmanager.EventResponse{
			{
				SenderId: 2,
				EventId:  idMap[2]["User2"],
				Time:     oneMonthLaterT.UnixMilli(),
				Name:     "User2",
			},
		},
		wantErr: nil,
	}, test{
		name: "Test 3",
		req: &eventmanager.GetEventsRequest{
			SenderId: 1,
			FromTime: oneMonthLaterT.AddDate(0, -2, 0).UnixMilli(),
			ToTime:   oneMonthLaterT.AddDate(0, -1, 0).UnixMilli(),
		},
		want:    []*eventmanager.EventResponse{},
		wantErr: fmt.Errorf("not found"),
	}, test{
		name: "Test 4",
		req: &eventmanager.GetEventsRequest{
			SenderId: 23432,
			FromTime: oneMonthLaterT.AddDate(0, -2, 0).UnixMilli(),
			ToTime:   oneMonthLaterT.AddDate(0, -1, 0).UnixMilli(),
		},
		want:    []*eventmanager.EventResponse{},
		wantErr: fmt.Errorf("not found senders events"),
	})

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			defer wg.Done()
			t.Run(tt.name, func(t *testing.T) {
				stream := makeStreamMock()
				err := s.GetEvents(tt.req, stream)

				if tt.wantErr == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, tt.wantErr.Error())
				}
				require.ElementsMatch(t, tt.want, stream.sent)
			})
		}(tt)
	}
	wg.Wait()
}

func TestServer_DeleteEvent(t *testing.T) {
	oneMonthLaterT := time.Now().AddDate(0, 1, 0)
	s, idMapPtr := setupTest(t, oneMonthLaterT)
	idMap := *idMapPtr

	type test struct {
		name    string
		req     *eventmanager.DeleteEventRequest
		want    *eventmanager.EventIdResponse
		wantErr error
	}
	var tests []test

	tests = append(tests, test{
		name: "Test 1",
		req: &eventmanager.DeleteEventRequest{
			SenderId: 7,
			EventId:  []byte("tyyh-8787"),
		},
		want:    nil,
		wantErr: fmt.Errorf("not found"),
	}, test{
		name: "Test 2",
		req: &eventmanager.DeleteEventRequest{
			SenderId: 1,
			EventId:  idMap[1]["User1"],
		},
		want:    &eventmanager.EventIdResponse{},
		wantErr: nil,
	}, test{
		name: "Test 3",
		req: &eventmanager.DeleteEventRequest{
			SenderId: 2,
			EventId:  idMap[2]["User2"],
		},
		want:    &eventmanager.EventIdResponse{},
		wantErr: nil,
	}, test{
		name: "Test 4",
		req: &eventmanager.DeleteEventRequest{
			SenderId: 3,
			EventId:  idMap[3]["User3"],
		},
		want:    &eventmanager.EventIdResponse{},
		wantErr: nil,
	}, test{
		name: "Test 5",
		req: &eventmanager.DeleteEventRequest{
			SenderId: 1,
			EventId:  []byte("13048007"),
		},
		want:    nil,
		wantErr: fmt.Errorf("not found"),
	})

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			defer wg.Done()
			t.Run(tt.name, func(t *testing.T) {
				got, err := s.DeleteEvent(context.Background(), tt.req)
				if tt.wantErr == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, tt.wantErr.Error())
				}
				if tt.want == nil {
					require.Nil(t, got)
				} else {
					actual := s.CheckSenderEventExistence(tt.req.SenderId, tt.req.EventId)
					require.Equal(t, got.EventId, tt.req.EventId)
					require.Equal(t, actual, false)
				}
			})
		}(tt)
	}
	wg.Wait()
}

func TestServer_passedEvents(t *testing.T) {
	oneMonthLaterT := time.Now().AddDate(0, 1, 0).UTC()
	type test struct {
		name    string
		timearg time.Time
		req     []*eventmanager.MakeEventRequest
		want    int
	}
	var tests []test

	var makeEventsArgs []*eventmanager.MakeEventRequest

	for i := 0; i < 10; i++ {
		makeEventsArgs = append(makeEventsArgs,
			&eventmanager.MakeEventRequest{
				SenderId: int64(i),
				Time:     oneMonthLaterT.AddDate(0, i, 0).UnixMilli(),
				Name:     fmt.Sprintf("User %d", i),
			},
		)
	}
	tests = append(tests,
		test{
			name:    "Test 1",
			timearg: oneMonthLaterT.AddDate(0, 5, 0),
			req:     makeEventsArgs,
			want:    4,
		}, test{
			name:    "Test 2",
			timearg: oneMonthLaterT.AddDate(1, 0, 0),
			req:     makeEventsArgs,
			want:    0,
		},
	)

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			defer wg.Done()
			t.Run(tt.name, func(t *testing.T) {
				s := NewServerEvent(tt.req...)
				s.passedEvents(tt.timearg)
				require.Equal(t, tt.want, s.eventsList.Len())
			})
		}(tt)
	}
	wg.Wait()
}

func TestServer_CheckSenderEventExistence(t *testing.T) {
	oneMonthLaterT := time.Now().AddDate(0, 1, 0)
	s, idMapPtr := setupTest(t, oneMonthLaterT)
	idMap := *idMapPtr

	type test struct {
		name      string
		sender_id int64
		event_id  []byte
		want      bool
	}
	var tests []test

	tests = append(tests, test{
		name:      "Test 1",
		sender_id: 1,
		event_id:  idMap[1]["User1"],
		want:      true,
	}, test{
		name:      "Test 2",
		sender_id: 2,
		event_id:  idMap[2]["User2"],
		want:      true,
	}, test{
		name:      "Test 3",
		sender_id: 2,
		event_id:  idMap[2]["User2Again"],
		want:      true,
	}, test{
		name:      "Test 4",
		sender_id: 7,
		event_id:  []byte("764tyy803323"),
		want:      false,
	}, test{
		name:      "Test 5",
		sender_id: 2,
		event_id:  []byte("10tytyhj0344"),
		want:      false,
	}, test{
		name:      "Test 6",
		sender_id: 3,
		event_id:  idMap[3]["User3"],
		want:      true,
	})

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			defer wg.Done()
			t.Run(tt.name, func(t *testing.T) {
				if got := s.CheckSenderEventExistence(tt.sender_id, tt.event_id); got != tt.want {
					t.Errorf("Server.CheckSenderEventExistence() = %v, want %v", got, tt.want)
				}
			})
		}(tt)
	}
	wg.Wait()
}

func TestArchiveEvent(t *testing.T) {
	currentTime := time.Now().UnixMilli()
	type test struct {
		name string
		req  models.Event
		want *eventmanager.EventResponse
	}
	var tests []test
	for i := 0; i < 100; i++ {
		eId := uuid.New()
		eByte, _ := eId.MarshalBinary()
		tests = append(tests, test{
			name: fmt.Sprintf("Test %d", i),
			req: models.Event{
				SenderId: int64(i),
				EventId:  eId,
				Time:     currentTime,
				Name:     fmt.Sprintf("User %d", i),
			},
			want: &eventmanager.EventResponse{
				SenderId: int64(i),
				EventId:  eByte,
				Time:     currentTime,
				Name:     fmt.Sprintf("User %d", i),
			},
		})
	}

	var wg sync.WaitGroup
	for _, tt := range tests {
		wg.Add(1)
		go func(tt test) {
			defer wg.Done()
			t.Run(tt.name, func(t *testing.T) {
				got := ArchiveEvent(tt.req)
				require.Equal(t, got, tt.want)
			})
		}(tt)
	}
	wg.Wait()
}
