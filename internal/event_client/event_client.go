package eventclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	"github.com/google/uuid"
)

type ClientManager interface {
	ClientMakeEvent(reader io.Reader, writer io.Writer) error
	ClientGetEvent(reader io.Reader, writer io.Writer) error
	ClientGetEvents(reader io.Reader, writer io.Writer) error
	ClientDeleteEvent(reader io.Reader, writer io.Writer) error
}

type manager struct {
	sender_id int64
	client    eventmanager.EventsClient
}

func newClientManager(id int64, cli eventmanager.EventsClient) *manager {
	return &manager{
		sender_id: id,
		client:    cli,
	}
}

func RunEventsClient(sender_id int64, client eventmanager.EventsClient, reader io.Reader, writer io.Writer) {
	routingKey := strconv.Itoa(int(sender_id))
	queueName := strconv.Itoa(int(sender_id))
	manager := newClientManager(sender_id, client)
	go notifyer(&eventmanager.EventResponse{}, routingKey, queueName)
	for {
		var call string
		fmt.Fscan(reader, &call)
		switch call {
		case "MakeEvent":
			{
				err := manager.ClientMakeEvent(reader, writer)
				if err != nil {
					fmt.Fprint(writer, err)
				}
			}
		case "GetEvent":
			{
				err := manager.ClientGetEvent(reader, writer)
				if err != nil {
					fmt.Fprint(writer, err)
				}
			}
		case "GetEvents":
			{
				err := manager.ClientGetEvents(reader, writer)
				if err != nil {
					fmt.Fprint(writer, err)
				}
			}
		case "DeleteEvent":
			{
				err := manager.ClientDeleteEvent(reader, writer)
				if err != nil {
					fmt.Fprint(writer, err)
				}
			}

		case "exit":
			return

		default:
			{
				fmt.Fprintln(writer, "undefined procedure name")
			}
		}
	}
}

func (m *manager) ClientMakeEvent(reader io.Reader, writer io.Writer) error {
	var event_time, event_date, name string
	_, err := fmt.Fscan(reader, &event_date, &event_time, &name)
	if err != nil {
		return errors.New("wrong number of arguments")
	}
	localTime, err := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", event_date, event_time), time.Local)
	if err != nil {
		return errors.New("wrong date format")
	}
	datetime := localTime.UTC()
	res, err := m.client.MakeEvent(context.Background(), &eventmanager.MakeEventRequest{
		SenderId: m.sender_id,
		Time:     datetime.UnixMilli(),
		Name:     name,
	})
	if err != nil {
		return err
	}
	eId, err := uuid.FromBytes(res.EventId)
	if err != nil {
		return err
	}
	fmt.Fprintln(writer, "eventId:", eId.String())
	return nil
}

func (m *manager) ClientGetEvent(reader io.Reader, writer io.Writer) error {
	var event_id string
	_, err := fmt.Fscan(reader, &event_id)
	if err != nil {
		return errors.New("wrong number of arguments")
	}
	eId, err := uuid.Parse(event_id)
	if err != nil {
		return errors.New("bad event id")
	}
	eIdBytes, _ := eId.MarshalBinary()
	res, err := m.client.GetEvent(context.Background(), &eventmanager.GetEventRequest{
		SenderId: m.sender_id,
		EventId:  eIdBytes,
	})
	if res == nil && err != nil {
		fmt.Fprintln(writer, "event not found")
		return nil
	}
	timeEvent := time.UnixMilli(res.Time).Local().Format(time.DateTime)
	eventId, _ := uuid.FromBytes(res.EventId)
	fmt.Fprintf(
		writer,
		"event {\n\tsenderId: %d\n\teventId: %s\n\ttime: %s\n\tname: %s\n}\n",
		res.SenderId,
		eventId.String(),
		timeEvent,
		res.Name,
	)
	return nil
}

func (m *manager) ClientGetEvents(reader io.Reader, writer io.Writer) error {
	var (
		fromDate string
		fromTime string
		toDate   string
		toTime   string
	)
	_, err := fmt.Fscan(reader, &fromDate, &fromTime, &toDate, &toTime)
	if err != nil {
		return errors.New("wrong number of arguments")
	}
	startTime, err := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", fromDate, fromTime), time.Local)
	if err != nil {
		return errors.New("wrong date format")
	}
	endTime, err := time.ParseInLocation(time.DateTime, fmt.Sprintf("%s %s", toDate, toTime), time.Local)
	if err != nil {
		return errors.New("wrong date format")
	}
	stream, err := m.client.GetEvents(context.Background(), &eventmanager.GetEventsRequest{
		SenderId: m.sender_id,
		FromTime: startTime.UTC().UnixMilli(),
		ToTime:   endTime.UTC().UnixMilli(),
	})
	if err != nil {
		fmt.Fprintln(writer, "events not found")
		return nil
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		t := time.UnixMilli(res.Time).Local().Format(time.DateTime)
		eventId, _ := uuid.FromBytes(res.EventId)
		fmt.Fprintf(writer, "event {\n\tsenderId: %d\n\teventId: %s\n\ttime: %s\n\tname: '%s'\n}\n", res.SenderId, eventId.String(), t, res.Name)
	}

	return nil
}

func (m *manager) ClientDeleteEvent(reader io.Reader, writer io.Writer) error {
	var event_id string
	_, err := fmt.Fscan(reader, &event_id)
	if err != nil {
		return errors.New("wrong number of arguments")
	}
	eId, err := uuid.Parse(event_id)
	if err != nil {
		return errors.New("bad event id")
	}
	eIdBytes, _ := eId.MarshalBinary()
	res, err := m.client.DeleteEvent(context.Background(), &eventmanager.DeleteEventRequest{
		SenderId: m.sender_id,
		EventId:  eIdBytes,
	})
	if err != nil {
		fmt.Fprintln(writer, "event not found")
	} else {
		eId, _ := uuid.FromBytes(res.EventId)
		fmt.Fprintln(writer, "eventId:", eId.String())
	}
	return nil
}
