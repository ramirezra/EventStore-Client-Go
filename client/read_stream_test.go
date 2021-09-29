package client_test

import (
	"context"
	"encoding/json"
	"github.com/EventStore/EventStore-Client-Go/client"
	"io"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/EventStore/EventStore-Client-Go/messages"
	uuid "github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Created struct {
	Seconds int64 `json:"seconds"`
	Nanos   int   `json:"nanos"`
}

type StreamRevision struct {
	Value uint64 `json:"value"`
}

type TestEvent struct {
	Event Event `json:"event"`
}

type Event struct {
	StreamID       string         `json:"streamId"`
	StreamRevision StreamRevision `json:"streamRevision"`
	EventID        uuid.UUID      `json:"eventId"`
	EventType      string         `json:"eventType"`
	EventData      []byte         `json:"eventData"`
	UserMetadata   []byte         `json:"userMetadata"`
	ContentType    string         `json:"contentType"`
	Position       Position       `json:"position"`
	Created        Created        `json:"created"`
}

func TestReadStreamEventsForwardsFromZeroPosition(t *testing.T) {
	eventsContent, err := ioutil.ReadFile("../resources/test/dataset20M-1800-e0-e10.json")
	require.NoError(t, err)

	var testEvents []TestEvent
	err = json.Unmarshal(eventsContent, &testEvents)
	require.NoError(t, err)

	container := GetPrePopulatedDatabase()
	defer container.Close()

	db := CreateTestClient(container, t)
	defer db.Close()

	context, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()

	numberOfEventsToRead := 10
	numberOfEvents := uint64(numberOfEventsToRead)

	streamId := "dataset20M-1800"

	opts := client.ReadStreamEventsOptions{}
	opts.SetForwards()
	opts.SetResolveLinks()

	stream, err := db.ReadStreamEvents(context, streamId, opts, numberOfEvents)

	if err != nil {
		t.Fatalf("Unexpected failure %+v", err)
	}

	defer stream.Close()

	events, err := collectStreamEvents(stream)

	if err != nil {
		t.Fatalf("Unexpected failure %+v", err)
	}

	assert.Equal(t, numberOfEvents, uint64(len(events)), "Expected the correct number of messages to be returned")

	for i := 0; i < numberOfEventsToRead; i++ {
		assert.Equal(t, testEvents[i].Event.EventID, events[i].GetOriginalEvent().EventID)
		assert.Equal(t, testEvents[i].Event.EventType, events[i].GetOriginalEvent().EventType)
		assert.Equal(t, testEvents[i].Event.StreamID, events[i].GetOriginalEvent().StreamID)
		assert.Equal(t, testEvents[i].Event.StreamRevision.Value, events[i].GetOriginalEvent().EventNumber)
		assert.Equal(t, testEvents[i].Event.Created.Nanos, events[i].GetOriginalEvent().CreatedDate.Nanosecond())
		assert.Equal(t, testEvents[i].Event.Created.Seconds, events[i].GetOriginalEvent().CreatedDate.Unix())
		assert.Equal(t, testEvents[i].Event.Position.Commit, events[i].GetOriginalEvent().Position.Commit)
		assert.Equal(t, testEvents[i].Event.Position.Prepare, events[i].GetOriginalEvent().Position.Prepare)
		assert.Equal(t, testEvents[i].Event.ContentType, events[i].GetOriginalEvent().ContentType)
	}
}

func TestReadStreamEventsBackwardsFromEndPosition(t *testing.T) {
	eventsContent, err := ioutil.ReadFile("../resources/test/dataset20M-1800-e1999-e1990.json")
	require.NoError(t, err)

	var testEvents []TestEvent
	err = json.Unmarshal(eventsContent, &testEvents)

	require.NoError(t, err)
	container := GetPrePopulatedDatabase()
	defer container.Close()

	db := CreateTestClient(container, t)
	defer db.Close()

	context, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()

	numberOfEventsToRead := 10
	numberOfEvents := uint64(numberOfEventsToRead)

	streamId := "dataset20M-1800"
	opts := client.ReadStreamEventsOptions{}
	opts.SetBackwards()
	opts.SetFromEnd()
	opts.SetResolveLinks()

	stream, err := db.ReadStreamEvents(context, streamId, opts, numberOfEvents)

	if err != nil {
		t.Fatalf("Unexpected failure %+v", err)
	}

	defer stream.Close()

	events, err := collectStreamEvents(stream)

	if err != nil {
		t.Fatalf("Unexpected failure %+v", err)
	}

	assert.Equal(t, numberOfEvents, uint64(len(events)), "Expected the correct number of messages to be returned")

	for i := 0; i < numberOfEventsToRead; i++ {
		assert.Equal(t, testEvents[i].Event.EventID, events[i].GetOriginalEvent().EventID)
		assert.Equal(t, testEvents[i].Event.EventType, events[i].GetOriginalEvent().EventType)
		assert.Equal(t, testEvents[i].Event.StreamID, events[i].GetOriginalEvent().StreamID)
		assert.Equal(t, testEvents[i].Event.StreamRevision.Value, events[i].GetOriginalEvent().EventNumber)
		assert.Equal(t, testEvents[i].Event.Created.Nanos, events[i].GetOriginalEvent().CreatedDate.Nanosecond())
		assert.Equal(t, testEvents[i].Event.Created.Seconds, events[i].GetOriginalEvent().CreatedDate.Unix())
		assert.Equal(t, testEvents[i].Event.Position.Commit, events[i].GetOriginalEvent().Position.Commit)
		assert.Equal(t, testEvents[i].Event.Position.Prepare, events[i].GetOriginalEvent().Position.Prepare)
		assert.Equal(t, testEvents[i].Event.ContentType, events[i].GetOriginalEvent().ContentType)
	}
}

func TestReadStreamReturnsEOFAfterCompletion(t *testing.T) {
	container := GetEmptyDatabase()
	defer container.Close()
	db := CreateTestClient(container, t)
	defer db.Close()

	var waitingForError sync.WaitGroup

	proposedEvents := []messages.ProposedEvent{}

	for i := 1; i <= 10; i++ {
		proposedEvents = append(proposedEvents, createTestEvent())
	}

	opts := client.AppendToStreamOptions{}
	opts.SetExpectNoStream()
	_, err := db.AppendToStream(context.Background(), "testing-closing", opts, proposedEvents...)
	require.NoError(t, err)

	stream, err := db.ReadStreamEvents(context.Background(), "testing-closing", client.ReadStreamEventsOptions{}, 1_024)

	require.NoError(t, err)
	_, err = collectStreamEvents(stream)
	require.NoError(t, err)

	go func() {
		_, err := stream.Recv()
		require.Error(t, err)
		require.True(t, err == io.EOF)
		waitingForError.Done()
	}()

	require.NoError(t, err)
	waitingForError.Add(1)
	timedOut := waitWithTimeout(&waitingForError, time.Duration(5)*time.Second)
	require.False(t, timedOut, "Timed out waiting for read stream to return io.EOF on completion")
}
