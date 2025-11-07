/*
Package awssqsconsumer handles a batch of AWS SQS messages by
accepting a user-defined function capable of handling a single message.

# Standard and FIFO Queues

The `awssqsconsumer` package provides support for both Standard and FIFO SQS queues.
[Standard] and [FIFO] accept an interface that provides the function [HandleMessage] to invoke on each message
and a function [WithConcurrentExecution] that indicates if the batch should be processed concurrently.

When using SQS as a Lambda event source, Lambda functions are triggered with a batch of messages.

If your Lambda function fails to process any message from the batch, the entire batch returns to your SQS.
This same batch is then retried until either condition happens first:
 - Your Lambda function returns a successful response.
 - Record reaches maximum retry attempts.
 - When records expire.

With this batch processor, batch records are processed individually - only messages that failed to be processed
return to SQS for further retry. This is done via [events.SQSEventResponse.BatchItemFailures].
*/
package awssqsconsumer

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

// MessageHandler is an interface that defines two functions.
// [HandleMessage] is a function that is invoked to process individual SQS message.
// [WithConcurrentExecution] is a function that determines if the batch of SQS messages should be processed concurrently.
type MessageHandler interface {
	HandleMessage(ctx context.Context, message events.SQSMessage) bool
	WithConcurrentExecution() bool
}

// StandardAdapter returns an AWS Lambda handler function for processing
// standard (non-FIFO) SQS queues using the provided MessageHandler.
//
// The returned function conforms to the AWS Lambda Go SDK signature
// `func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error)`.
//
// When invoked by Lambda, each SQS message from the SQSEvent is passed to
// the given handler for processing. The interface's WithConcurrentExecution()
// method determines whether messages are processed concurrently or
// sequentially.
//
// Example usage:
//
//	lambda.Start(awssqsconsumer.StandardAdapter(context.TODO(), &MyHandler{}))
//
// StandardAdapter delegates message processing to the Standard function.
func StandardAdapter(ctx context.Context, handler MessageHandler) func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	return func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
		return Standard(ctx, handler, event)
	}
}

// FIFOAdapter returns an AWS Lambda handler function for processing
// FIFO (First-In, First-Out) SQS queues using the provided MessageHandler.
//
// The returned function conforms to the AWS Lambda Go SDK signature
// `func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error)`.
//
// It delegates message handling to the FIFO function, which
// should guarantee ordered and exactly-once processing behavior as
// defined by AWS SQS FIFO queue semantics.
//
// Example usage:
//
//	lambda.Start(awssqsconsumer.FIFOAdapter(context.TODO(), &MyFIFOHandler{}))
func FIFOAdapter(ctx context.Context, handler MessageHandler) func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	return func(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
		return FIFO(ctx, handler, event)
	}
}

type HandleMessageFunc func(ctx context.Context, message events.SQSMessage) bool
