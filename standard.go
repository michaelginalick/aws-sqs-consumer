package awssqsconsumer

import (
	"context"
	"sync"

	"github.com/aws/aws-lambda-go/events"
)

// Standard is an AWS Lambda function handler capable of handling
// messages from an AWS SQS Standard Queue
func Standard(ctx context.Context, handler MessageHandler, event events.SQSEvent) (events.SQSEventResponse, error) {
	if handler.WithConcurrentExecution() {
		return handleStandardConcurrently(ctx, handler.HandleMessage, event)
	}

	return handleStandard(ctx, handler.HandleMessage, event)
}

func handleStandardConcurrently(ctx context.Context, handler HandleMessageFunc, event events.SQSEvent) (events.SQSEventResponse, error) {
	var response events.SQSEventResponse
	response.BatchItemFailures = make([]events.SQSBatchItemFailure, 0, len(event.Records))
	ch := make(chan events.SQSBatchItemFailure)
	var wg sync.WaitGroup

	for _, message := range event.Records {
		wg.Add(1)
		go func(message events.SQSMessage) {
			defer wg.Done()
			if ok := handler(ctx, message); !ok {
				ch <- events.SQSBatchItemFailure{ItemIdentifier: message.MessageId}
			}
		}(message)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for failure := range ch {
		response.BatchItemFailures = append(response.BatchItemFailures, failure)
	}
	return response, nil
}

func handleStandard(ctx context.Context, handler HandleMessageFunc, event events.SQSEvent) (events.SQSEventResponse, error) {
	var response events.SQSEventResponse
	response.BatchItemFailures = make([]events.SQSBatchItemFailure, 0, len(event.Records))

	for _, message := range event.Records {
		if ok := handler(ctx, message); !ok {
			response.BatchItemFailures = append(
				response.BatchItemFailures,
				events.SQSBatchItemFailure{
					ItemIdentifier: message.MessageId,
				})
		}

	}
	return response, nil
}
