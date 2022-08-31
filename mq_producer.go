package fit

import (
	"errors"
	"github.com/nsqio/go-nsq"
)

var producerConfig *nsq.Config

type Producer struct {
	Producer *nsq.Producer
}

func NewProducerConfig() {
	producerConfig = nsq.NewConfig()
}

// NewProducer create new a nsq producer
func NewProducer(addr string) (*Producer, error) {
	if len(addr) == 0 {
		return nil, errors.New("find not producer address")
	}

	var p Producer
	producer, err := nsq.NewProducer(addr, producerConfig)
	if err != nil {
		return nil, err
	}
	p.Producer = producer
	return &p, nil
}

// Put push message to topic
func (p *Producer) Put(topic string, body []byte) error {
	if len(topic) == 0 || len(body) == 0 {
		return errors.New("topic cannot be empty")
	}
	if err := p.Producer.Publish(topic, body); err != nil {
		return err
	}
	p.Producer.Stop()
	return nil
}
