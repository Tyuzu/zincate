package mq

// import (
// 	"log"
// 	"sync"
// 	"time"

// 	"github.com/streadway/amqp"
// )

// // ConnectToRabbitMQ establishes a connection to RabbitMQ and returns the connection and channel.
// func ConnectToRabbitMQ(url string) (*amqp.Connection, *amqp.Channel, error) {
// 	conn, err := amqp.Dial(url)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	ch, err := conn.Channel()
// 	if err != nil {
// 		conn.Close()
// 		return nil, nil, err
// 	}

// 	return conn, ch, nil
// }

// // DeclareQueue declares a queue in RabbitMQ and returns it.
// func DeclareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
// 	queue, err := ch.QueueDeclare(
// 		name,  // name
// 		true,  // durable
// 		false, // delete when unused
// 		false, // exclusive
// 		false, // no-wait
// 		nil,   // arguments
// 	)
// 	if err != nil {
// 		return amqp.Queue{}, err
// 	}
// 	return queue, nil
// }

// // PublishMessage sends a message to the specified queue.
// func PublishMessage(ch *amqp.Channel, queueName string, message string) error {
// 	return ch.Publish(
// 		"",        // exchange
// 		queueName, // routing key
// 		false,     // mandatory
// 		false,     // immediate
// 		amqp.Publishing{
// 			ContentType: "text/plain",
// 			Body:        []byte(message),
// 		},
// 	)
// }

// // WorkerPool initializes a worker pool to consume and process RabbitMQ messages.
// func WorkerPool(
// 	numWorkers int,
// 	msgs <-chan amqp.Delivery,
// 	processFunc func(body []byte),
// ) {
// 	var wg sync.WaitGroup
// 	wg.Add(numWorkers)

// 	for i := 0; i < numWorkers; i++ {
// 		go func(workerID int) {
// 			defer wg.Done()
// 			for msg := range msgs {
// 				log.Printf("Worker %d: Received message: %s", workerID, msg.Body)
// 				processFunc(msg.Body)
// 			}
// 		}(i)
// 	}

// 	wg.Wait()
// }

// // ExampleProcessingFunction simulates message processing logic (replace with your custom logic).
// func ExampleProcessingFunction(body []byte) {
// 	log.Printf("Processing message: %s", body)
// 	time.Sleep(2 * time.Second) // Simulate work
// 	log.Printf("Finished processing message: %s", body)
// }

// /*
// package main

// import (
// 	"log"

// 	"./rabbitmq" // Adjust the import path to your project structure
// )

// func main() {
// 	// RabbitMQ connection setup
// 	rabbitMQURL := "amqp://guest:guest@localhost:5672/"
// 	conn, ch, err := rabbitmq.ConnectToRabbitMQ(rabbitMQURL)
// 	if err != nil {
// 		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
// 	}
// 	defer conn.Close()
// 	defer ch.Close()

// 	// Declare queue
// 	queueName := "worker_pool_queue"
// 	queue, err := rabbitmq.DeclareQueue(ch, queueName)
// 	if err != nil {
// 		log.Fatalf("Failed to declare queue: %v", err)
// 	}

// 	// Publish some test messages
// 	for i := 1; i <= 10; i++ {
// 		message := fmt.Sprintf("Task #%d", i)
// 		err := rabbitmq.PublishMessage(ch, queue.Name, message)
// 		if err != nil {
// 			log.Printf("Failed to publish message: %v", err)
// 		}
// 	}

// 	// Consume messages
// 	msgs, err := ch.Consume(
// 		queue.Name, // queue
// 		"",         // consumer
// 		true,       // auto-ack
// 		false,      // exclusive
// 		false,      // no-local
// 		false,      // no-wait
// 		nil,        // args
// 	)
// 	if err != nil {
// 		log.Fatalf("Failed to register consumer: %v", err)
// 	}

// 	// Start worker pool
// 	numWorkers := 5
// 	rabbitmq.WorkerPool(numWorkers, msgs, rabbitmq.ExampleProcessingFunction)
// }
// */
