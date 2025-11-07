# AWS SQS Consumer

AWSSQSConsumer is a small, flexible library that normalizes your application consumes [AWS SQS messages](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html). It handles a batch of SQS messages by accepting a user-defined function capable of handling a single message.

### About

The `awssqsconsumer` package provides support for both Standard and FIFO SQS queues.
[Standard] and [FIFO] accept an interface that provides the function [HandleMessage] to invoke on each message
and a function [WithConcurrentExecution] that indicates if the batch should be processed concurrently.

When using SQS as a Lambda event source, Lambda functions are triggered with a batch of messages.

If your Lambda function fails to process any message from the batch, the entire batch returns to your SQS.
This same batch is then retried until either condition happens first:
 - Your Lambda function returns a successful response.
 - Record reaches maximum retry attempts.
 - When records expire.

With this batch processor, batch records are processed individually - only messages that failed to be processed return to SQS for further retry.


## Features

**awssqsconsumer** is a flexible, lightweight package for processing SQS messages in lambda applications.
It is able to process both [FIFO](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-fifo-queues.html) and [Standard](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/standard-queues.html) queues. It also provides the option to process both queue types using lock free concurrency.