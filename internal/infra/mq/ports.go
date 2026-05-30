// Package mq 定义消息队列的抽象端口接口。
//
// 领域层通过这些接口发布消息，具体的 RabbitMQ 或其他 MQ 实现负责消息投递。
// 消费者只有在 handler 返回 nil 后才能确认消息，以确保消息处理的可靠性。
package mq

import "context"

// Message 定义 MQ 消息结构，包含消息 ID、目标队列、幂等键、业务载荷和自定义头信息。
type Message struct {
	ID             string
	Queue          string
	IdempotencyKey string
	Payload        []byte
	Headers        map[string]string
}

// Publisher 定义消息发布接口，由 MQ adapter 实现。
type Publisher interface {
	Publish(ctx context.Context, queue string, message Message) error
}

// Consumer 定义消息消费接口，由 MQ adapter 实现。
// handler 处理完消息后返回 nil 才确认消息。
type Consumer interface {
	Consume(ctx context.Context, queue string, handler func(context.Context, Message) error) error
}
