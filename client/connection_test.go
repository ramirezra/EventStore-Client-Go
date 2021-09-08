package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/EventStore/EventStore-Client-Go/messages"
	stream_revision "github.com/EventStore/EventStore-Client-Go/streamrevision"
	"github.com/EventStore/EventStore-Client-Go/options"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_CloseConnection(t *testing.T) {
	container := GetEmptyDatabase()
	defer container.Close()

	client := CreateTestClient(container, t)

	testEvent := messages.ProposedEvent{
		EventID:      uuid.FromStringOrNil("38fffbc2-339e-11ea-8c7b-784f43837872"),
		EventType:    "TestEvent",
		ContentType:  "application/octet-stream",
		UserMetadata: []byte{0xd, 0xe, 0xa, 0xd},
		Data:         []byte{0xb, 0xe, 0xe, 0xf},
	}
	proposedEvents := []messages.ProposedEvent{
		testEvent,
	}

	streamID, _ := uuid.NewV4()
	context, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	opts := options.NewAppendToStreamOptions().ExpectedRevision(stream_revision.NoStream())
	_, err := client.AppendToStream(context, streamID.String(), opts, proposedEvents)

	if err != nil {
		t.Fatalf("Unexpected failure %+v", err)
	}

	client.Close()
	opts = options.NewAppendToStreamOptions()
	_, err = client.AppendToStream(context, streamID.String(), opts, proposedEvents)

	assert.NotNil(t, err)
	assert.Equal(t, "esdb connection is closed", err.Error())
}
