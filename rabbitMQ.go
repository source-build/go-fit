package fit

import (
	"errors"
	"fmt"
	"github.com/streadway/amqp"
)

var MQURL string

const (
	ALL         = iota     //write in all ways according to the log configuration
	LOCAL                  //local log
	REMOTE                 //remote log，Take effect after configuring log
	CONSOLE                //output to console
	KIND_FANOUT = "fanout" //post messages to all queues bound to this switch

	// KIND_DIRECT deliver the message to the queue where the bindingkey and routingkey exactly match
	KIND_DIRECT = "direct"

	// KIND_TOPIC rule matching. There are two special characters in bindingkey* Match zero or more words,
	//35; match one word
	KIND_TOPIC = "topic"

	// KIND_HEADER it does not depend on the routingkey, but performs matching binding through the headers attribute in the message body.
	//The key in the headers and the bindingkey are completely matched
	KIND_HEADER = "header"
)

var errHandles []int

type PublishConfig struct {
	Exchange  string
	Key       string
	Mandatory bool
	Immediate bool
	Msg       amqp.Publishing
}

type ConsumeConfig struct {
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      amqp.Table
}

type RabbitMQ struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	errHandles   []int
	Queue        amqp.Queue
	ExchangeName string
	Key          string
	MqURL        string
}

//func NewRabbitMQ(queueName string, exchange string, key string) *RabbitMQ {
//	return &RabbitMQ{QueueName: queueName, Exchange: exchange, Key: key, MqURL: MQURL}
//}

// SetRabbitMqErrLogHandle Optional value
// ALL LOCAL REMOTE
func SetRabbitMqErrLogHandle(v ...int) {
	errHandles = v
}

func SetMqURL(url string) {
	MQURL = url
}

func (r *RabbitMQ) Channel() *amqp.Channel {
	return r.channel
}

func (r *RabbitMQ) Conn() *amqp.Connection {
	return r.conn
}

func (r *RabbitMQ) Close() {
	err := r.channel.Close()
	if err != nil {
		r.failOnErr("mq channel close failed err:", err)
	}
	err = r.conn.Close()
	if err != nil {
		r.failOnErr("mq conn close failed err:", err)
	}
}

func (r *RabbitMQ) failOnErr(msg string, err error) {
	if r.errHandles != nil {
		for _, kv := range r.errHandles {
			switch kv {
			case ALL:
				Error("msg", msg, "err", err)
				return
			case CONSOLE:
				fmt.Println(msg, err)
			case LOCAL:
				LocalLog().Error("msg", msg, "err", err)
			case REMOTE:
				RemoteLog(ErrorLevel, "msg", msg, "err", err)
			}
		}
		return
	}
	if errHandles != nil {
		errHandles = append(errHandles, LOCAL)
	}
}

func (r *RabbitMQ) SetRabbitMqErrLogHandle(v ...int) {
	r.errHandles = v
}

func (r *RabbitMQ) QueueDeclare(name string, durable bool, autoDelete bool, exclusive bool, noWait bool, args amqp.Table) *RabbitMQ {
	r.Queue, _ = r.channel.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
	return r
}

func (r *RabbitMQ) DefQueueDeclare(name string, durable, autoDel bool) *RabbitMQ {
	r.Queue, _ = r.channel.QueueDeclare(name, durable, autoDel, false, false, nil)
	return r
}

func (r *RabbitMQ) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) *RabbitMQ {
	err := r.channel.ExchangeDeclare(
		name,
		kind,
		durable,
		autoDelete,
		internal,
		noWait,
		args,
	)
	if err != nil {
		r.failOnErr("ExchangeDeclare fail", err)
	}

	r.ExchangeName = name
	return r
}

func (r *RabbitMQ) DefExchangeDeclare(name, kind string, durable, autoDel bool) *RabbitMQ {
	err := r.channel.ExchangeDeclare(
		name,
		kind,
		durable,
		autoDel,
		false,
		false,
		nil,
	)
	if err != nil {
		r.failOnErr("ExchangeDeclare fail", err)
	}

	r.ExchangeName = name
	return r
}

func (r *RabbitMQ) Consume(v []ConsumeConfig) (<-chan amqp.Delivery, error) {
	if len(v) > 0 {
		conf := v[0]
		return r.channel.Consume(
			r.Queue.Name,
			conf.Consumer,
			conf.AutoAck,
			conf.Exclusive,
			conf.NoLocal,
			conf.NoWait,
			conf.Args,
		)
	}

	return r.channel.Consume(
		r.Queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
}

func NewRabbitMQ() (*RabbitMQ, error) {
	if len(MQURL) == 0 {
		return nil, errors.New("MqURL cannot be empty")
	}

	rabbitmq := &RabbitMQ{MqURL: MQURL}
	var err error
	rabbitmq.conn, err = amqp.Dial(rabbitmq.MqURL)
	if err != nil {
		rabbitmq.failOnErr("rabbitMq Dial an error occurred", err)
		return nil, err
	}

	rabbitmq.channel, err = rabbitmq.conn.Channel()
	if err != nil {
		rabbitmq.failOnErr("rabbitmq.conn.Channel() an error occurred", err)
		return nil, err
	}
	return rabbitmq, nil
}

func (r *RabbitMQ) PublishSimple(message string) error {
	if len(r.Queue.Name) == 0 {
		return errors.New("please first declare queue")
	}

	return r.channel.Publish(
		"",
		r.Queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
}

func (r *RabbitMQ) ConsumeSimple(v ...ConsumeConfig) (<-chan amqp.Delivery, error) {
	if len(r.Queue.Name) == 0 {
		return nil, errors.New("please first declare queue")
	}

	return r.Consume(v)
}

func (r *RabbitMQ) Publish(message, key string) error {
	if len(r.ExchangeName) == 0 {
		return errors.New("please first declare exchange")
	}

	return r.channel.Publish(
		r.ExchangeName,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
}

func (r *RabbitMQ) PublishPub(message string) error {
	if len(r.ExchangeName) == 0 {
		return errors.New("please first declare exchange")
	}

	return r.channel.Publish(
		r.ExchangeName,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
}

func (r *RabbitMQ) PublishRouting(message string) error {
	if len(r.ExchangeName) == 0 {
		return errors.New("please first declare exchange")
	}

	return r.channel.Publish(
		r.ExchangeName,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
}

func (r *RabbitMQ) PublishTopic(message, key string) error {
	if len(r.ExchangeName) == 0 {
		return errors.New("please first declare exchange")
	}

	return r.channel.Publish(
		r.ExchangeName,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
}

func (r *RabbitMQ) ReceiveSub(v ...ConsumeConfig) (<-chan amqp.Delivery, error) {
	if len(r.ExchangeName) == 0 {
		return nil, errors.New("please first declare exchange")
	}
	if len(r.Queue.Name) == 0 {
		return nil, errors.New("please first declare queue")
	}

	err := r.channel.QueueBind(
		r.Queue.Name,
		"",
		r.ExchangeName,
		false,
		nil)
	if err != nil {
		return nil, err
	}

	return r.Consume(v)
}

func (r *RabbitMQ) ReceiveRouting(key string, v ...ConsumeConfig) (<-chan amqp.Delivery, error) {
	if len(r.ExchangeName) == 0 {
		return nil, errors.New("please first declare exchange")
	}

	err := r.channel.QueueBind(
		r.Queue.Name,
		key,
		r.ExchangeName,
		false,
		nil)
	if err != nil {
		return nil, err
	}

	return r.Consume(v)
}

func (r *RabbitMQ) ReceiveTopic(key string, v ...ConsumeConfig) (<-chan amqp.Delivery, error) {
	if len(r.ExchangeName) == 0 {
		return nil, errors.New("please first declare exchange")
	}
	if len(r.Queue.Name) == 0 {
		return nil, errors.New("please first declare queue")
	}

	err := r.channel.QueueBind(
		r.Queue.Name,
		key,
		r.ExchangeName,
		false,
		nil)
	if err != nil {
		return nil, err
	}

	return r.Consume(v)
}
