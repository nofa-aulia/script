package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func main() {
	topicArn := "arn:aws:sns:ap-southeast-1:107126629234:everwhatsapp-official_meta-messaging-webhook_prod.fifo"
	subject := "meta_messaging_webhook"
	region := "ap-southeast-1"
	accessKey := "<access_key>"   // TODO: Replace with actual access key
	secretKey := "<secret_key>"   // TODO: Replace with actual secret key
	csvFile := "webhook_logs.csv" // TODO: Replace with actual CSV file path

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	snsClient := sns.NewFromConfig(cfg)

	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatalf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("failed to read CSV file: %v", err)
	}

	for i, record := range records {
		if i == 0 {
			continue
		}

		if len(record) < 3 {
			log.Printf("skipping row %d: not enough columns", i)
			continue
		}

		id := record[0]
		externalMessageId := record[1]
		payload := record[2]

		messageGroupId := fmt.Sprintf("meta_webhook.messages-%s", id)

		var messageMap map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &messageMap); err != nil {
			log.Printf("failed to unmarshal message for row %d: %v", i, err)
			continue
		}
		jsonMessage, err := json.Marshal(messageMap)
		if err != nil {
			log.Printf("failed to marshal message for row %d: %v", i, err)
			continue
		}

		input := &sns.PublishInput{
			Message:        aws.String(string(jsonMessage)),
			TopicArn:       aws.String(topicArn),
			MessageGroupId: aws.String(messageGroupId),
		}

		if subject != "" {
			input.Subject = aws.String(subject)
		}

		result, err := snsClient.Publish(ctx, input)
		if err != nil {
			log.Printf("failed to publish message for row %d: %v", i, err)
			continue
		}

		fmt.Printf("Row %d: Message published successfully!\n", i)
		fmt.Printf("Row %d: Message ID: %s\n", i, *result.MessageId)
		fmt.Printf("Row %d: External Message Id: %s, ID: %s\n", i, externalMessageId, id)
		fmt.Printf("Row %d: Message: %s\n", i, string(jsonMessage))
		fmt.Printf("Row %d: Message Group ID: %s\n", i, messageGroupId)
		fmt.Println("-----")
		fmt.Println("")

		time.Sleep(1 * time.Second)
	}
}
