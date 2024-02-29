package eventclient

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
)

func RunEventsClient(sender_id int64, client eventmanager.EventsClient) {
	routingKey := strconv.Itoa(int(sender_id))
	queueName := strconv.Itoa(int(sender_id))

	go notifyer(&eventmanager.GetEventsResponse{}, routingKey, queueName)

	for {
		var call string
		fmt.Scan(&call)
		switch call {
		case "MakeEvent":
			{
				var event_time, event_date, name string
				fmt.Scan(&event_date, &event_time, &name)
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
				fmt.Println("eventId: ", res.EventId)
			}
		case "GetEvent":
			{
				var event_id int64
				fmt.Scan(&event_id)
				res, err := client.GetEvent(context.Background(), &eventmanager.GetEventRequest{
					SenderId: sender_id,
					EventId:  event_id,
				})
				if err != nil {
					fmt.Println("NotFound")
				}
				timeEvent := time.UnixMilli(res.Time).Local().Format(time.DateTime)
				fmt.Printf("Event {\n\tsenderId: %d\n\teventId: %d\n\ttime: %s\n\tname: %s\n}\n", res.SenderId, res.EventId, timeEvent, res.Name)
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
				startTime, _ := time.Parse(time.DateTime, fmt.Sprintf("%s %s", fromDate, fromTime))
				endTime, _ := time.Parse(time.DateTime, fmt.Sprintf("%s %s", toDate, toTime))

				stream, err := client.GetEvents(context.Background(), &eventmanager.GetEventsRequest{
					SenderId: sender_id,
					FromTime: startTime.UnixMilli(),
					ToTime:   endTime.UnixMilli(),
				})
				if err != nil {
					fmt.Println("NotFound")
				} else {
					for i := 0; ; i++ {
						res, err := stream.Recv()
						if err == io.EOF {
							if i == 0 {
								fmt.Println("Not found")
							}
							break
						}
						t := time.UnixMilli(res.Time).Local().Format(time.DateTime)
						fmt.Printf("Event {\n\tsenderId: %d\n\teventId: %d\n\ttime: %s\n\tname: '%s'\n}\n", res.SenderId, res.EventId, t, res.Name)
					}
				}
			}
		case "DeleteEvent":
			{
				var event_id int64
				fmt.Scan(&event_id)
				res, err := client.DeleteEvent(context.Background(), &eventmanager.DeleteEventRequest{
					SenderId: sender_id,
					EventId:  event_id,
				})
				if err != nil {
					fmt.Println("NotFound")
				}
				fmt.Println("eventId: ", res.EventId)
			}

		case "exit":
			return
		}
	}

}
