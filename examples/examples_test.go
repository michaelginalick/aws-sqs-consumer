package awssqsconsumer_test

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awssqsconsumer "github.com/michaelginalick/aws-sqs-consumer"
)

type standardHandler struct{}

func (h *standardHandler) WithConcurrentExecution() bool {
	return true
}

func (h *standardHandler) HandleMessage(ctx context.Context, message events.SQSMessage) bool {
	fmt.Println(message.Body)
	return true
}

type FIFOHandler struct{}

func (fh *FIFOHandler) HandleMessage(ctx context.Context, message events.SQSMessage) bool {
	return true
}

func (fh *FIFOHandler) WithConcurrentExecution() bool {
	return false
}

// The ExampleStandardAdapter demostrates how to consume a Standard SQS queue using the StandardAdapter
func ExampleStandardAdapter() {
	lambda.Start(awssqsconsumer.StandardAdapter(context.TODO(), &standardHandler{}))
}

// The ExampleFIFOAdapter demostrates how to consume a FIFO SQS queue using the FIFOAdapter
func ExampleFIFOAdapter() {
	lambda.Start(awssqsconsumer.FIFOAdapter(context.TODO(), &FIFOHandler{}))
}

// The Example demostrates how to use the MessageHandler interface without using the
// provided adapter functions
func Example() {
	handler := func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
		return awssqsconsumer.Standard(ctx, &FIFOHandler{}, event)
	}

	lambda.Start(handler)
}
