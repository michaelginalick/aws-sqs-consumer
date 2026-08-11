package awssqsconsumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awssqsconsumer "github.com/michaelginalick/aws-sqs-consumer"
)

// contextKey is an unexported type used as a context key to avoid collisions
// with keys from other packages.
type contextKey int

const (
	loggerKey contextKey = iota
	dbKey
	ledgerKey
)

// OrderEvent is the domain payload carried inside each SQS message body.
type OrderEvent struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	TotalCents int64  `json:"total_cents"`
}

// PaymentEvent is sequenced per customer; order matters, so we use a FIFO queue.
type PaymentEvent struct {
	PaymentID   string `json:"payment_id"`
	CustomerID  string `json:"customer_id"`
	AmountCents int64  `json:"amount_cents"`
}

// DB is a stand-in for whatever data-access layer your service uses.
type DB interface {
	SaveOrder(ctx context.Context, e OrderEvent) error
}

// Ledger records payments in strict order.
type Ledger interface {
	RecordPayment(ctx context.Context, e PaymentEvent) error
}

// Typed accessors panic with a clear message at startup (inside lambda.Start)
// if the caller forgot to attach a dependency, rather than crashing silently
// on the first message in production.
func loggerFromCtx(ctx context.Context) *slog.Logger {
	v, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok || v == nil {
		panic("awssqsconsumer: logger not set in context — attach it with context.WithValue before starting the handler")
	}
	return v
}

func dbFromCtx(ctx context.Context) DB {
	v, ok := ctx.Value(dbKey).(DB)
	if !ok || v == nil {
		panic("awssqsconsumer: DB not set in context — attach it with context.WithValue before calling StandardAdapter")
	}
	return v
}

func ledgerFromCtx(ctx context.Context) Ledger {
	v, ok := ctx.Value(ledgerKey).(Ledger)
	if !ok || v == nil {
		panic("awssqsconsumer: Ledger not set in context — attach it with context.WithValue before calling FIFOAdapter")
	}
	return v
}

// OrderHandler processes order events from a Standard SQS queue.
// Messages within a batch are handled concurrently; each goroutine
// receives the same base context so it can propagate the Lambda
// deadline and access injected dependencies.
type OrderHandler struct{}

func (h *OrderHandler) WithConcurrentExecution() bool { return true }

func (h *OrderHandler) HandleMessage(ctx context.Context, msg events.SQSMessage) bool {
	logger := loggerFromCtx(ctx)
	db := dbFromCtx(ctx)

	var order OrderEvent
	if err := json.Unmarshal([]byte(msg.Body), &order); err != nil {
		logger.Error("failed to unmarshal order event",
			slog.String("message_id", msg.MessageId),
			slog.Any("error", err),
		)
		// Return false so this message is added to BatchItemFailures and
		// returned to the queue for retry.
		return false
	}

	// Respect the Lambda invocation deadline: if the context is already
	// cancelled before we reach the DB call, bail out immediately so the
	// message is retried rather than silently dropped.
	select {
	case <-ctx.Done():
		logger.Warn("context cancelled before DB write",
			slog.String("order_id", order.OrderID),
			slog.Any("reason", context.Cause(ctx)),
		)
		return false
	default:
	}

	if err := db.SaveOrder(ctx, order); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logger.Warn("DB write interrupted",
				slog.String("order_id", order.OrderID),
				slog.Any("reason", err),
			)
			return false
		}
		logger.Error("failed to save order",
			slog.String("order_id", order.OrderID),
			slog.Any("error", err),
		)
		return false
	}

	logger.Info("order processed", slog.String("order_id", order.OrderID))
	return true
}

// PaymentHandler processes payment events from a FIFO SQS queue.
// Sequential execution is required: a failed payment must block all
// subsequent payments for the same customer (message group).
type PaymentHandler struct{}

func (h *PaymentHandler) WithConcurrentExecution() bool { return false }

func (h *PaymentHandler) HandleMessage(ctx context.Context, msg events.SQSMessage) bool {
	logger := loggerFromCtx(ctx)
	ledger := ledgerFromCtx(ctx)

	var payment PaymentEvent
	if err := json.Unmarshal([]byte(msg.Body), &payment); err != nil {
		logger.Error("failed to unmarshal payment event",
			slog.String("message_id", msg.MessageId),
			slog.Any("error", err),
		)
		return false
	}

	if err := ledger.RecordPayment(ctx, payment); err != nil {
		logger.Error("failed to record payment",
			slog.String("payment_id", payment.PaymentID),
			slog.String("customer_id", payment.CustomerID),
			slog.Any("error", err),
		)
		// Returning false stops the group here. The library marks this
		// message and every subsequent message in the same group as failed
		// so they are retried in order.
		return false
	}

	logger.Info("payment recorded",
		slog.String("payment_id", payment.PaymentID),
		slog.String("customer_id", payment.CustomerID),
	)
	return true
}

// ExampleStandardAdapter demonstrates how to consume a Standard SQS queue
// using StandardAdapter. Dependencies are attached to a base context and
// forwarded to every HandleMessage call.
func ExampleStandardAdapter() {
	logger := slog.Default()
	db := newDB()

	baseCtx := context.WithValue(context.Background(), loggerKey, logger)
	baseCtx = context.WithValue(baseCtx, dbKey, db)

	lambda.Start(awssqsconsumer.StandardAdapter(baseCtx, &OrderHandler{}))
}

// ExampleFIFOAdapter demonstrates how to consume a FIFO SQS queue using
// FIFOAdapter. Sequential execution preserves message-group ordering.
func ExampleFIFOAdapter() {
	logger := slog.Default()
	ledger := newLedger()

	baseCtx := context.WithValue(context.Background(), loggerKey, logger)
	baseCtx = context.WithValue(baseCtx, ledgerKey, ledger)

	lambda.Start(awssqsconsumer.FIFOAdapter(baseCtx, &PaymentHandler{}))
}

// ExampleStandard demonstrates how to call Standard directly without the
// adapter, useful when wrapping the handler with middleware. The Lambda-
// provided context is forwarded so handlers see the invocation deadline.
func ExampleStandard() {
	logger := slog.Default()
	db := newDB()

	handler := func(lambdaCtx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
		// Attach dependencies to the Lambda-provided context so handlers
		// see both the invocation deadline and the injected values.
		ctx := context.WithValue(lambdaCtx, loggerKey, logger)
		ctx = context.WithValue(ctx, dbKey, db)
		return awssqsconsumer.Standard(ctx, &OrderHandler{}, event)
	}

	lambda.Start(handler)
}

func newDB() DB         { panic("replace with real DB init") }
func newLedger() Ledger { panic("replace with real ledger init") }
