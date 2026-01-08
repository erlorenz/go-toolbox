package pubsub_test

import (
	"testing"

	"github.com/erlorenz/go-toolbox/pubsub"
)

func TestInMemory(t *testing.T) {
	testBroker(t, func() pubsub.Broker {
		return pubsub.NewInMemory()
	}, nil)
}

func BenchmarkInMemoryPublish_NoSubscribers(b *testing.B) {
	broker := pubsub.NewInMemory()
	defer broker.Close()
	benchmarkPublish(b, broker, 0)
}

func BenchmarkInMemoryPublish_1Subscriber(b *testing.B) {
	broker := pubsub.NewInMemory()
	defer broker.Close()
	benchmarkPublish(b, broker, 1)
}

func BenchmarkInMemoryPublish_10Subscribers(b *testing.B) {
	broker := pubsub.NewInMemory()
	defer broker.Close()
	benchmarkPublish(b, broker, 10)
}

func BenchmarkInMemoryPublish_100Subscribers(b *testing.B) {
	broker := pubsub.NewInMemory()
	defer broker.Close()
	benchmarkPublish(b, broker, 100)
}
