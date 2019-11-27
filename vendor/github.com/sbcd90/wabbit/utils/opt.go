package utils

import (
	"fmt"

	"github.com/sbcd90/wabbit"
	"github.com/streadway/amqp"
)

type xstring []string

var (
	amqpOptions xstring = []string{
		"headers",
		"contentType",
		"contentEncoding",
		"deliveryMode",
		"priority",
		"messageId",
	}
)

func (s xstring) Contains(key string) bool {
	for _, v := range s {
		if key == v {
			return true
		}
	}

	return false
}

func ConvertOpt(opt wabbit.Option) (amqp.Publishing, error) {
	var (
		headers         = amqp.Table{}
		contentType     = "text/plain"
		contentEncoding = ""
		deliveryMode    = amqp.Transient
		priority        = uint8(0)
		messageId       = ""
	)

	if wrongOpt, ok := checkOptions(opt); !ok {
		return amqp.Publishing{}, fmt.Errorf("Wring option '%s'. Check the docs.", wrongOpt)
	}

	if opt != nil {
		if h, ok := opt["headers"].(amqp.Table); ok {
			headers = h
		}

		if c, ok := opt["contentType"].(string); ok {
			contentType = c
		}

		if c, ok := opt["contentEncoding"].(string); ok {
			contentEncoding = c
		}

		if d, ok := opt["deliveryMode"].(uint8); ok {
			deliveryMode = d
		}

		if p, ok := opt["priority"].(uint8); ok {
			priority = p
		}

		if p, ok := opt["messageId"].(string); ok {
			messageId = p
		}
	}

	return amqp.Publishing{
		Headers:         headers,
		ContentType:     contentType,
		ContentEncoding: contentEncoding,
		DeliveryMode:    deliveryMode, // 1=non-persistent, 2=persistent
		Priority:        priority,     // 0-9
		MessageId:       messageId,
		// a bunch of application/implementation-specific fields
	}, nil
}

func checkOptions(opt wabbit.Option) (string, bool) {
	optMap := map[string]interface{}(opt)

	for k, _ := range optMap {
		if !amqpOptions.Contains(k) {
			return k, false
		}
	}

	return "", true
}
