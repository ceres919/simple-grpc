package eventclient

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	"github.com/google/uuid"
)

type ClientManager interface {
	ClientMakeEvent(client eventmanager.EventsClient)
	ClientGetEvent(client eventmanager.EventsClient)
	ClientGetEvents(client eventmanager.EventsClient)
	ClientDeleteEvent(client eventmanager.EventsClient)
}

type manager struct {
}

func RunEventsClient(sender_id int64, client eventmanager.EventsClient) {
	routingKey := strconv.Itoa(int(sender_id))
	queueName := strconv.Itoa(int(sender_id))
	manager := manager{}
	go notifyer(&eventmanager.EventResponse{}, routingKey, queueName)

	for {
		var call string
		fmt.Scan(&call)
		switch call {
		case "MakeEvent":
			{
				var event_time, event_date, name string
				fmt.Scan(&event_date, &event_time, &name)
				manager.ClientMakeEvent(client, sender_id, event_time, event_date, name)
				//eventsClientMakeEvent(client, sender_id, event_time, event_date, name)
				// localTime, _ := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", event_date, event_time), time.Local)
				// datetime := localTime.UTC()
				// res, err := client.MakeEvent(context.Background(), &eventmanager.MakeEventRequest{
				// 	SenderId: sender_id,
				// 	Time:     datetime.UnixMilli(),
				// 	Name:     name,
				// })
				// if err != nil {
				// 	log.Fatal(err)
				// }
				// fmt.Println("eventId: ", res.EventId)
			}
		case "GetEvent":
			{
				var event_id string
				fmt.Scan(&event_id)

				manager.ClientGetEvent(client, sender_id, event_id)
				//eventsClientGetEvent(client, sender_id, event_id)
				// res, _ := client.GetEvent(context.Background(), &eventmanager.GetEventRequest{
				// 	SenderId: sender_id,
				// 	EventId:  event_id,
				// })
				// if res == nil {
				// 	fmt.Println("No such event")
				// } else {
				// 	timeEvent := time.UnixMilli(res.Time).Local().Format(time.DateTime)
				// 	fmt.Printf("Event {\n\tsenderId: %d\n\teventId: %d\n\ttime: %s\n\tname: %s\n}\n", res.SenderId, res.EventId, timeEvent, res.Name)
				// }
			}
		case "GetEvents":
			{
				var (
					fromDate string
					fromTime string
					toDate   string
					toTime   string
				)
				fmt.Scan(&fromDate, &fromTime, &toDate, &toTime)
				manager.ClientGetEvents(client, sender_id, fromDate, fromTime, toDate, toTime)
				//eventsClientGetEvents(client, sender_id, fromDate, fromTime, toDate, toTime)
				// startTime, _ := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", fromDate, fromTime), time.Local)
				// endTime, _ := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", toDate, toTime), time.Local)

				// stream, err := client.GetEvents(context.Background(), &eventmanager.GetEventsRequest{
				// 	SenderId: sender_id,
				// 	FromTime: startTime.UTC().UnixMilli(),
				// 	ToTime:   endTime.UTC().UnixMilli(),
				// })
				// if err != nil {
				// 	fmt.Println("No such events")
				// } else {
				// 	for i := 0; ; i++ {
				// 		res, err := stream.Recv()
				// 		if err == io.EOF {
				// 			if i == 0 {
				// 				fmt.Println("No such events")
				// 			}
				// 			break
				// 		}
				// 		t := time.UnixMilli(res.Time).Local().Format(time.DateTime)
				// 		fmt.Printf("Event {\n\tsenderId: %d\n\teventId: %d\n\ttime: %s\n\tname: '%s'\n}\n", res.SenderId, res.EventId, t, res.Name)
				// 	}
				// }
			}
		case "DeleteEvent":
			{
				var event_id string
				fmt.Scan(&event_id)
				manager.ClientDeleteEvent(client, sender_id, event_id)
				//eventsClientDeleteEvent(client, sender_id, event_id)
				// res, err := client.DeleteEvent(context.Background(), &eventmanager.DeleteEventRequest{
				// 	SenderId: sender_id,
				// 	EventId:  event_id,
				// })
				// if err != nil {
				// 	fmt.Println("Not Found")
				// }
				// fmt.Println("eventId: ", res.EventId)
			}

		case "exit":
			return
		}
	}

}

func (m *manager) ClientMakeEvent(client eventmanager.EventsClient, sender_id int64, event_time, event_date, name string) {
	localTime, _ := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", event_date, event_time), time.Local)
	datetime := localTime.UTC()
	res, err := client.MakeEvent(context.Background(), &eventmanager.MakeEventRequest{
		SenderId: sender_id,
		Time:     datetime.UnixMilli(),
		Name:     name,
	})
	if err != nil {
		log.Fatal(err)
	}
	eUuid, _ := uuid.FromBytes(res.EventId)
	fmt.Println("eventId: ", eUuid.String())
}

func (m *manager) ClientGetEvent(client eventmanager.EventsClient, sender_id int64, event_id string) {
	eId, _ := uuid.Parse(event_id)
	eIdBytes, _ := eId.MarshalBinary()
	res, _ := client.GetEvent(context.Background(), &eventmanager.GetEventRequest{
		SenderId: sender_id,
		EventId:  eIdBytes,
	})
	if res == nil {
		fmt.Println("No such event")
	} else {
		timeEvent := time.UnixMilli(res.Time).Local().Format(time.DateTime)
		eventId, _ := uuid.FromBytes(res.EventId)
		fmt.Printf("Event {\n\tsenderId: %d\n\teventId: %s\n\ttime: %s\n\tname: %s\n}\n", res.SenderId, eventId.String(), timeEvent, res.Name)
	}
}

func (m *manager) ClientGetEvents(
	client eventmanager.EventsClient,
	sender_id int64,
	fromDate string,
	fromTime string,
	toDate string,
	toTime string,
) {
	startTime, _ := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", fromDate, fromTime), time.Local)
	endTime, _ := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", toDate, toTime), time.Local)

	stream, err := client.GetEvents(context.Background(), &eventmanager.GetEventsRequest{
		SenderId: sender_id,
		FromTime: startTime.UTC().UnixMilli(),
		ToTime:   endTime.UTC().UnixMilli(),
	})
	if err != nil {
		fmt.Println("No such events")
	} else {
		for i := 0; ; i++ {
			res, err := stream.Recv()
			if err == io.EOF {
				if i == 0 {
					fmt.Println("No such events")
				}
				break
			}
			t := time.UnixMilli(res.Time).Local().Format(time.DateTime)
			eventId, _ := uuid.FromBytes(res.EventId)
			fmt.Printf("Event {\n\tsenderId: %d\n\teventId: %s\n\ttime: %s\n\tname: '%s'\n}\n", res.SenderId, eventId.String(), t, res.Name)
		}
	}
}

func (m *manager) ClientDeleteEvent(client eventmanager.EventsClient, sender_id int64, event_id string) {
	eId, _ := uuid.Parse(event_id)
	eIdBytes, _ := eId.MarshalBinary()
	res, err := client.DeleteEvent(context.Background(), &eventmanager.DeleteEventRequest{
		SenderId: sender_id,
		EventId:  eIdBytes,
	})
	if err != nil {
		fmt.Println("Not Found")
	}
	eUuid, _ := uuid.FromBytes(res.EventId)
	fmt.Println("eventId: ", eUuid.String())
}
