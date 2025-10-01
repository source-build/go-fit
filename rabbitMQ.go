package fit

import (
	"errors"

	"github.com/rabbitmq/amqp091-go"
)

var rabbitMQUrl string

type RabbitMQ struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
}

func GlobalSetRabbitMQUrl(url string) {
	rabbitMQUrl = url
}

func (r *RabbitMQ) Channel() *amqp091.Channel {
	return r.channel
}

func (r *RabbitMQ) Conn() *amqp091.Connection {
	return r.conn
}

func (r *RabbitMQ) Close() error {
	err := r.channel.Close()
	if err != nil {
		return err
	}

	err = r.conn.Close()
	if err != nil {
		return err
	}

	return nil
}

func NewRabbitMQ(mqUrl ...string) (*RabbitMQ, error) {
	if rabbitMQUrl == "" && len(mqUrl) == 0 {
		return nil, errors.New("mq url cannot be empty")
	}

	url := rabbitMQUrl

	if len(mqUrl) > 0 && mqUrl[0] != "" {
		url = mqUrl[0]
	}

	rabbitmq := &RabbitMQ{}
	var err error

	rabbitmq.conn, err = amqp091.Dial(url)
	if err != nil {
		return nil, err
	}

	rabbitmq.channel, err = rabbitmq.conn.Channel()
	if err != nil {
		return nil, err
	}

	return rabbitmq, nil
}
