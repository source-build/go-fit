package fit

import (
	"github.com/nsqio/go-nsq"
	"time"
)

type ConsumerEntity struct {
	Topic   string
	Channel string
	Address string
	Handler nsq.Handler
}

func InitConsumer(entity ConsumerEntity) (err error) {
	config := nsq.NewConfig()
	config.LookupdPollInterval = 15 * time.Second
	c, err := nsq.NewConsumer(entity.Topic, entity.Channel, config)
	if err != nil {
		return err
	}

	c.AddHandler(entity.Handler)

	if err := c.ConnectToNSQLookupd(entity.Address); err != nil {
		return err
	}
	return nil
}
