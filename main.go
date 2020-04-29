package main

import (
	"context"
	"flag"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

var (
	srcURI    = flag.String("src-uri", "", `Source URI e.g. amqp://username:password@rabbitmq-fqdn:5672`)
	srcQueue  = flag.String("src-queue", "", `Source queue name`)
	dstURI    = flag.String("dst-uri", "", `Destination URI e.g. amqp://username:password@rabbitmq-fqdn:5672`)
	dstQueue  = flag.String("dst-queue", "", `Destination queue name`)
	limit     = flag.Int("limit", 0, "Limit the number of messages")
	tx        = flag.Bool("tx", false, `Use producer transactions (slow)`)
)

type moveCommand struct {
	Source struct {
		URI   string
		Queue string
	}
	Destination struct {
		URI   string
		Queue string
	}
	Limit int
	Tx    bool
}

func (s *moveCommand) Run(ctx context.Context) error {
	if err := s.Validate(); err != nil {
		return err
	}

	consumerConn, err := amqp.Dial(s.Source.URI)
	if err != nil {
		return errors.Wrapf(err, "Cannot connect consumer to '%s'", s.Source.URI)
	}
	//noinspection ALL
	defer consumerConn.Close()

	consumerChannel, err := consumerConn.Channel()
	if err != nil {
		return errors.Wrapf(err, "Cannot create consumer channel")
	}
	//noinspection ALL
	defer consumerChannel.Close()

	producerConn, err := amqp.Dial(s.Destination.URI)
	if err != nil {
		return errors.Wrapf(err, "Cannot connect producer to '%s'", s.Destination.URI)
	}
	//noinspection ALL
	defer producerConn.Close()

	producerChannel, err := producerConn.Channel()
	if err != nil {
		return errors.Wrapf(err, "Cannot create producer channel")
	}
	//noinspection ALL
	defer producerChannel.Close()

	consumerQueue, err := consumerChannel.QueueDeclarePassive(
		s.Source.Queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return errors.Wrapf(err, "Cannot describe consumer queue '%s'", s.Source.Queue)
	}

	producerQueue, err := producerChannel.QueueDeclarePassive(
		s.Destination.Queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return errors.Wrapf(err, "Cannot describe producer queue '%s'", s.Destination.Queue)
	}

	log.Infof("Number of messages in consumer queue %s: %d", s.Source.Queue, consumerQueue.Messages)
	log.Infof("Number of messages in producer queue %s: %d", s.Destination.Queue, producerQueue.Messages)

	//
	if consumerQueue.Messages == 0 && s.Limit >= 0 {
		log.Infof("Consumer queue is empty")
		return nil
	}
	limit := s.Limit
	if s.Limit == 0 {
		limit = consumerQueue.Messages
	}
	log.Infof("Moving messages from %s to %s with limit %d", s.Source.Queue, s.Destination.Queue, limit)
	return s.move(ctx, consumerChannel, producerChannel, limit)
}

func (s *moveCommand) move(ctx context.Context, consumerChannel, producerChannel *amqp.Channel, limit int) error {

	msgs, err := consumerChannel.Consume(
		s.Source.Queue, // queue
		"rabbitmq-mv",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return errors.Wrapf(err, "Cannot consume from queue '%s'", s.Source.Queue)
	}
	if s.Tx {
		// this will be slow ...
		err = producerChannel.Tx()
		if err != nil {
			return errors.Wrapf(err, "Cannot open producer transaction")
		}
	}

	var counter int
consumeLoop:
	for {
		select {
		case d := <-msgs:
			err = producerChannel.Publish(
				"",
				s.Destination.Queue,
				true,
				false,
				amqp.Publishing{
					Body:            d.Body,
					Headers:         d.Headers,
					ContentType:     d.ContentType,
					ContentEncoding: d.ContentEncoding,
					DeliveryMode:    d.DeliveryMode,
					Priority:        d.Priority,
					CorrelationId:   d.CorrelationId,
					ReplyTo:         d.ReplyTo,
					Expiration:      d.Expiration,
					MessageId:       d.MessageId,
					Timestamp:       d.Timestamp,
					Type:            d.Type,
					UserId:          d.UserId,
					AppId:           d.AppId,
				})
			if err != nil {
				return err
			}
			if s.Tx {
				err = producerChannel.TxCommit()
				if err != nil {
					return errors.Wrapf(err, "Cannot commit producer transaction")
				}
			}
			err = consumerChannel.Ack(d.DeliveryTag, false)
			if err != nil {
				return errors.Wrapf(err, "Cannot ack consumer message")
			}
			counter++
			if limit >= 0 && counter >= limit {
				break consumeLoop
			}
		case <-time.After(1 * time.Second):
			if limit >= 0 {
				break consumeLoop
			}
			log.Infof("Moved so far %d messages ", counter)
		case <-ctx.Done():
			log.Warnf("Move interrupted after %d messages ", counter)
			break consumeLoop
		}
	}
	log.Infof("Moved %d messages ", counter)
	return nil
}

func (s *moveCommand) Validate() error {
	if s.Source.URI == "" {
		return errors.New("Source URI must not be empty")
	}
	if s.Source.Queue == "" {
		return errors.New("Source Queue must not be empty")
	}
	if s.Destination.URI == "" {
		return errors.New("Destination URI must not be empty")
	}
	if s.Destination.Queue == "" {
		return errors.New("Destination Queue must not be empty")
	}
	if s.Source.URI == s.Destination.URI && s.Source.Queue == s.Destination.Queue {
		return errors.New("Source and Destination are the same")
	}
	return nil
}

func newMoveCommand() (*moveCommand, error) {
	c := moveCommand{}
	c.Source.URI = *srcURI
	c.Source.Queue = *srcQueue
	c.Destination.URI = *dstURI
	c.Destination.Queue = *dstQueue
	c.Limit = *limit
	c.Tx = *tx

	if c.Destination.URI == "" {
		c.Destination.URI = c.Source.URI
	}
	if c.Source.URI == "" {
		c.Source.URI = c.Destination.URI
	}

	return &c, nil
}

func main() {
	flag.Parse()

	start := time.Now()
	defer func() {
		log.Infof("Operation took %v", time.Now().Sub(start))
	}()

	moveCommand, err := newMoveCommand()
	if err != nil {
		log.Fatalf("%v", err)
	}

	shutdownCtx, shutdownFunc := context.WithCancel(context.Background())
	defer shutdownFunc()

	var g run.Group
	g.Add(func() error {
		defer shutdownFunc()
		handleSigterm(shutdownCtx)
		return nil
	}, func(error) {
	})

	g.Add(func() error {
		defer shutdownFunc()
		return moveCommand.Run(shutdownCtx)
	}, func(error) {
	})
	err = g.Run()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func handleSigterm(ctx context.Context) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		log.Debugf("shutdown received ...")
	case q := <-quit:
		log.Infof("quit %v received ... ", q)
	}
}
