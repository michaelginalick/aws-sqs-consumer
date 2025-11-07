package awssqsconsumer_test

import (
	"context"
	"sort"
	"strconv"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	awssqsconsumer "github.com/michaelginalick/aws-sqs-consumer"
	assert "github.com/michaelginalick/aws-sqs-consumer/internal"
)

func sortItems(items events.SQSEventResponse) []events.SQSBatchItemFailure {
	sort.Slice(items.BatchItemFailures, func(i, j int) bool {
		id1, _ := strconv.Atoi(items.BatchItemFailures[i].ItemIdentifier)
		id2, _ := strconv.Atoi(items.BatchItemFailures[j].ItemIdentifier)
		return id1 < id2
	})
	return items.BatchItemFailures
}

type nonConcurrentFIFO struct{}

func (p *nonConcurrentFIFO) WithConcurrentExecution() bool {
	return false
}

func (p *nonConcurrentFIFO) HandleMessage(ctx context.Context, message events.SQSMessage) bool {
	return message.Body != "invalid Event"
}

type concurrentFIFO struct{}

func (p *concurrentFIFO) WithConcurrentExecution() bool {
	return true
}

func (p *concurrentFIFO) HandleMessage(ctx context.Context, message events.SQSMessage) bool {
	return message.Body != "invalid Event"
}

func TestFIFO(t *testing.T) {

	tests := []struct {
		name                 string
		event                events.SQSEvent
		nonConcurrenthandler awssqsconsumer.MessageHandler
		concurrentHandler    awssqsconsumer.MessageHandler
		want                 events.SQSEventResponse
	}{
		{
			name: "SuccessfulEvent",
			event: events.SQSEvent{
				Records: []events.SQSMessage{
					{
						MessageId: "1",
						Body:      "success",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
					{
						MessageId: "2",
						Body:      "success",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
					{
						MessageId: "3",
						Body:      "success",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
				},
			},
			nonConcurrenthandler: &nonConcurrentFIFO{},
			concurrentHandler:    &concurrentFIFO{},
			want: events.SQSEventResponse{
				BatchItemFailures: []events.SQSBatchItemFailure{},
			},
		},
		{
			name: "FailedEvent",
			event: events.SQSEvent{
				Records: []events.SQSMessage{
					{
						MessageId: "1",
						Body:      "invalid Event",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
					{
						MessageId: "2",
						Body:      "invalid Event",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
					{
						MessageId: "3",
						Body:      "invalid Event",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
				},
			},
			nonConcurrenthandler: &nonConcurrentFIFO{},
			concurrentHandler:    &concurrentFIFO{},
			want: events.SQSEventResponse{
				BatchItemFailures: []events.SQSBatchItemFailure{
					{
						ItemIdentifier: "1",
					},
					{
						ItemIdentifier: "2",
					},
					{
						ItemIdentifier: "3",
					},
				},
			},
		},
		{
			name: "MixedEvent",
			event: events.SQSEvent{
				Records: []events.SQSMessage{
					{
						MessageId: "1",
						Body:      "valid message",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
					{
						MessageId: "2",
						Body:      "invalid Event",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
					{
						MessageId: "3",
						Body:      "valid message",
						Attributes: map[string]string{
							"MessageGroupId": "A",
						},
					},
				},
			},
			nonConcurrenthandler: &nonConcurrentFIFO{},
			concurrentHandler:    &concurrentFIFO{},
			want: events.SQSEventResponse{
				BatchItemFailures: []events.SQSBatchItemFailure{
					{
						ItemIdentifier: "2",
					},
					{
						ItemIdentifier: "3",
					},
				},
			},
		},
	}

	t.Run("Non-Concurrent", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := awssqsconsumer.FIFO(context.Background(), tt.nonConcurrenthandler, tt.event)
				assert.Nil(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("Concurrent", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := awssqsconsumer.FIFO(context.Background(), tt.concurrentHandler, tt.event)
				assert.Nil(t, err)
				assert.Equal(t, sortItems(tt.want), sortItems(got))
			})
		}
	})

}
