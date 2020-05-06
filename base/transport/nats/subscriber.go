package nats

import (
	"context"
	kitep "github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/transport"
	kitn "github.com/go-kit/kit/transport/nats"
	natn "github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/vtomar01/go-base/base/endpoint"
	"github.com/vtomar01/go-base/base/log"
)

type (

	// Decoder decodes the message received on NATS and converts into business entity
	Decoder func(context.Context, *natn.Msg) (request interface{}, err error)

	// ResponseHandler handles the endpoint response
	ResponseHandler func(context.Context, string, *natn.Conn, interface{}) error

	BeforeFunc func(context.Context, *natn.Msg) context.Context

	AfterFunc func(context.Context, *natn.Conn) context.Context

	ErrorEncoder kitn.ErrorEncoder

	ErrorHandler interface{ transport.ErrorHandler }

	// Subscriber is NATS subscription
	Subscriber struct {
		*kitn.Subscriber

		id       string
		subject  string
		qGroup   string
		end      endpoint.Endpoint
		dec      Decoder
		reshn    ResponseHandler
		befores  []BeforeFunc
		afters   []AfterFunc
		errorEnc ErrorEncoder
		errorhn  ErrorHandler

		subscription *natn.Subscription
		options      []kitn.SubscriberOption
	}

	// SubscriberOption provides set of options to modify a Subscriber
	SubscriberOption func(*Subscriber)
)

func WithQGroupSubscriberOption(qGroup string) SubscriberOption {
	return func(s *Subscriber) {
		s.qGroup = qGroup
	}
}

func (s *Subscriber) Id() string {
	return s.id
}

func (s *Subscriber) Topic() string {
	return s.subject
}

func (s *Subscriber) Group() string {
	return s.qGroup
}

func (s *Subscriber) IsValid() bool {
	return s.subscription.IsValid()
}

func WithSubjectSubscriberOption(sub string) SubscriberOption {
	return func(s *Subscriber) {
		s.subject = sub
	}
}

func WithEndpointSubscriberOption(end endpoint.Endpoint) SubscriberOption {
	return func(s *Subscriber) {
		s.end = end
	}
}

func WithDecoderSubscriberOption(fn Decoder) SubscriberOption {
	return func(s *Subscriber) {
		s.dec = fn
	}
}

func WithBeforeFuncsSubscriberOption(fns ...BeforeFunc) SubscriberOption {
	return func(s *Subscriber) {
		s.befores = append(s.befores, fns...)
		for _, fn := range fns {
			s.options = append(
				s.options,
				kitn.SubscriberBefore(kitn.RequestFunc(fn)),
			)
		}
	}
}

func WithAfterFuncsSubscriberOption(fns ...AfterFunc) SubscriberOption {
	return func(s *Subscriber) {
		s.afters = append(s.afters, fns...)
		for _, fn := range fns {
			s.options = append(
				s.options,
				kitn.SubscriberAfter(kitn.SubscriberResponseFunc(fn)),
			)
		}
	}
}

func WithErrorEncoderSubscriberOption(e ErrorEncoder) SubscriberOption {
	return func(s *Subscriber) {
		s.errorEnc = e
		s.options = append(
			s.options,
			kitn.SubscriberErrorEncoder(kitn.ErrorEncoder(e)),
		)
	}
}

func WithErrorhandlerSubscriberOption(e ErrorHandler) SubscriberOption {
	return func(s *Subscriber) {
		s.errorhn = e
		s.options = append(s.options, kitn.SubscriberErrorHandler(e))
	}
}

func (s *Subscriber) open(con *natn.Conn) error {

	var err error
	if len(s.qGroup) > 0 {
		s.subscription, err = con.QueueSubscribe(
			s.subject,
			s.qGroup,
			s.ServeMsg(con),
		)
		s.subscription.IsValid()
	} else {
		s.subscription, err = con.Subscribe(
			s.subject,
			s.ServeMsg(con),
		)
	}

	return err
}

func (s *Subscriber) close() error {
	return s.subscription.Drain()
}

func newSubscriber(
	logger log.Logger,
	options ...SubscriberOption,
) (*Subscriber, error) {

	var s Subscriber

	for _, o := range options {
		o(&s)
	}

	if s.end == nil {
		return nil, errors.Wrap(
			ErrCreatingSubscriber, "missing endpoint",
		)
	}

	if len(s.subject) == 0 {
		return nil, errors.Wrap(
			ErrCreatingSubscriber, "missing subject",
		)
	}

	if s.dec == nil {
		return nil, errors.Wrap(
			ErrCreatingSubscriber, "missing decoder",
		)
	}

	if s.reshn == nil {
		s.reshn = NoOpResponseHandler
	}

	if s.errorEnc == nil {
		WithErrorEncoderSubscriberOption(NoOpErrorEncoder)
	}

	if s.errorhn == nil {
		WithErrorhandlerSubscriberOption(transport.NewLogErrorHandler(logger))
	}

	s.Subscriber = kitn.NewSubscriber(
		kitep.Endpoint(s.end),
		kitn.DecodeRequestFunc(s.dec),
		kitn.EncodeResponseFunc(s.reshn),
		s.options...,
	)

	return &s, nil
}
