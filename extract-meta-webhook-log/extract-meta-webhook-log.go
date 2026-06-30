package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

type Payload struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					WAId string `json:"wa_id"`
				} `json:"contacts"`
				Statuses []struct {
					ID          string `json:"id"`
					Status      string `json:"status"`
					Reason      string `json:"reason"`
					Timestamp   string `json:"timestamp"`
					RecipientId string `json:"recipient_id"`
					Errors      []struct {
						Message   string `json:"message"`
						ErrorData struct {
							Details string `json:"details"`
						} `json:"error_data"`
					} `json:"errors"`
				} `json:"statuses"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

type MessageStatus struct {
	Id                string `json:"id"`
	ReqID             string `json:"reqID"`
	ExternalMessageID string `json:"external_message_id"`
	To                string `json:"to"`
}

func loadMessageStatusMap(filePath string) map[string]MessageStatus {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open message status file: %v", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		log.Fatalf("Failed to read message status CSV: %v", err)
	}

	result := make(map[string]MessageStatus)
	if len(records) == 0 {
		return result
	}

	colIdx := make(map[string]int)
	for i, col := range records[0] {
		colIdx[col] = i
	}

	required := []string{"id", "reqID", "external_message_id", "to"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			log.Fatalf("column '%s' not found in message status file", col)
		}
	}

	for _, row := range records[1:] {
		extMsgID := row[colIdx["external_message_id"]]
		if extMsgID == "" {
			continue
		}
		result[extMsgID] = MessageStatus{
			Id:                row[colIdx["id"]],
			ReqID:             row[colIdx["reqID"]],
			ExternalMessageID: extMsgID,
			To:                row[colIdx["to"]],
		}
	}
	return result
}

func main() {
	webhookRequestLogFile := "webhook_request_log.csv"
	messageStatusFile := "message_status.csv"
	outputFile := "output_webhook_logs.csv"

	outputColumns := []string{
		"message_status_id",     // id from message status data
		"external_message_id",   // external message id
		"payload",               // webhook payload
		"webhook_status",        // message status from webhook log
		"error_message",         // error message from webhook log
		"error_detail",          // error detail from webhook log
		"webhook_recipient_id",  // recipient id (customer phone number) from webhook log
		"message_status_to",     // "to" (customer phone number) from message status data
		"message_status_req_id", // reqID from message status data
		"timestamp",             // timestamp (epoch) from webhook log
		"timestamp_utc"}         // timestamp (utc) from webhook log

	msgStatusMap := loadMessageStatusMap(messageStatusFile)
	fmt.Printf("Loaded %d entries from message status file\n", len(msgStatusMap))

	// Open input file
	inFile, err := os.Open(webhookRequestLogFile)
	if err != nil {
		log.Fatalf("Failed to open input file: %v", err)
	}
	defer inFile.Close()

	// Create CSV reader
	reader := csv.NewReader(inFile)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV: %v", err)
	}

	if len(records) == 0 {
		log.Fatal("CSV file is empty")
	}

	var outputRecords [][]string
	outputRecords = append(outputRecords, outputColumns)

	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 5 {
			continue
		}

		payloadStr := row[4] // payload is at index 4

		var payload Payload
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			continue
		}

		if len(payload.Entry) == 0 || len(payload.Entry[0].Changes) == 0 {
			continue
		}

		value := payload.Entry[0].Changes[0].Value

		statuses := value.Statuses
		if len(statuses) == 0 {
			continue
		}

		externalMessageID := statuses[0].ID
		if _, exists := msgStatusMap[externalMessageID]; !exists {
			continue
		}

		recipientID := statuses[0].RecipientId

		errorMessage := ""
		errorDetail := ""
		if len(statuses[0].Errors) > 0 {
			errorMessage = statuses[0].Errors[0].Message
			errorDetail = statuses[0].Errors[0].ErrorData.Details
		}

		timestamp := statuses[0].Timestamp
		timestampUTC := ""
		if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
			timestampUTC = time.Unix(ts, 0).UTC().Format("2006-01-02T15:04:05Z")
		}
		status := statuses[0].Status

		msgStatus := msgStatusMap[externalMessageID]

		outputRow := []string{
			msgStatus.Id,      // message_status_id
			externalMessageID, // external_message_id
			payloadStr,        // payload
			status,            // webhook_status
			errorMessage,      // error_message
			errorDetail,       // error_detail
			recipientID,       // webhook_recipient_id
			msgStatus.To,      // message_status_to
			msgStatus.ReqID,   // message_status_req_id
			timestamp,         // timestamp
			timestampUTC,      // timestamp_utc
		}
		outputRecords = append(outputRecords, outputRow)
	}

	// Write output file
	outFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	for _, record := range outputRecords {
		if err := writer.Write(record); err != nil {
			log.Fatalf("Failed to write record: %v", err)
		}
	}

	fmt.Printf("Records written: %d. Output: %s\n", len(outputRecords)-1, outputFile)
}
