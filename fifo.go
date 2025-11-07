package awssqsconsumer

import (
	"context"
	"sync"

	"github.com/aws/aws-lambda-go/events"
)

// FIFO is function capable of handling messages from AWS FIFO Queue.
func FIFO(ctx context.Context, handler MessageHandler, event events.SQSEvent) (events.SQSEventResponse, error) {
	if handler.WithConcurrentExecution() {
		return handleFIFOConcurrently(ctx, handler.HandleMessage, event)
	}

	return handleFIFO(ctx, handler.HandleMessage, event)
}

func handleFIFOConcurrently(ctx context.Context, handler HandleMessageFunc, event events.SQSEvent) (events.SQSEventResponse, error) {
	var response events.SQSEventResponse
	response.BatchItemFailures = make([]events.SQSBatchItemFailure, 0, len(event.Records))
	messageGroups := make(map[string][]events.SQSMessage)
	for _, message := range event.Records {
		groupID := message.Attributes["MessageGroupId"]
		messageGroups[groupID] = append(messageGroups[groupID], message)
	}

	ch := make(chan events.SQSBatchItemFailure)
	var wg sync.WaitGroup
	for _, group := range messageGroups {
		wg.Add(1)
		go func(group []events.SQSMessage) {
			defer wg.Done()
			i := 0
			for ; i < len(group); i++ {
				message := group[i]
				// Stop processing messages after first failure.
				// Return all failed and unprocessed messages in SQSBatchItemFailure
				if ok := handler(ctx, message); !ok {
					break
				}
			}

			for ; i < len(group); i++ {
				message := group[i]
				failure := events.SQSBatchItemFailure{
					ItemIdentifier: message.MessageId,
				}
				ch <- failure
			}
		}(group)
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

func handleFIFO(ctx context.Context, handler HandleMessageFunc, event events.SQSEvent) (events.SQSEventResponse, error) {
	var response events.SQSEventResponse
	response.BatchItemFailures = make([]events.SQSBatchItemFailure, 0, len(event.Records))

	messageGroups := make(map[string][]events.SQSMessage)
	for _, message := range event.Records {
		groupID := message.Attributes["MessageGroupId"]
		messageGroups[groupID] = append(messageGroups[groupID], message)
	}

	for _, group := range messageGroups {
		i := 0
		for ; i < len(group); i++ {
			message := group[i]
			// Stop processing messages after first failure.
			// Return all failed and unprocessed messages in SQSBatchItemFailure
			if ok := handler(ctx, message); !ok {
				break
			}
		}

		for ; i < len(group); i++ {
			message := group[i]
			failure := events.SQSBatchItemFailure{
				ItemIdentifier: message.MessageId,
			}
			response.BatchItemFailures = append(response.BatchItemFailures, failure)
		}
	}

	return response, nil
}
