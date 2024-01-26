package main

import (
	"fmt" // Import the "fmt" package for printing
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type CampaignView struct {
	Id              uint   `json:"id" gorm:"primary_key"`
	OrgId           uint   `json:"-"`
	PartialShopName string `json:"shop_name" gorm:"column:shop_name; not null"`
	SiteVisitorId   string `json:"site_visitor_id"`
	CartToken       string `json:"cart_token"`
	CampaignId      uint   `json:"campaign_id"`
	ProductId       uint   `json:"product_id"`
	PlaybookId      uint   `json:"playbook_id"`

	CreatedAt int64 `json:"timestamp" gorm:"column:created_at; not null"`
}

type CampaignViewCount struct {
	Shop       string `json:"shop"`
	CampaignId int    `json:"campaign_id"`
	ViewCount  int    `json:"view_count"`
	PlaybookId int    `json:"playbook_id"`
}

type ShopifyCartCreate struct {
	Id             int64  `json:"id" gorm:"primary_key"`
	CartToken      string `json:"cart_token"`
	Shop           string `json:"shop"`
	BlobCachedJson []byte `json:"blob_cached_json"`
	CreatedAt      int64  `json:"created_at"`
}

type ShopifyOrderCreate struct {
	ID             int64  `json:"id"`
	CartToken      string `json:"cart_token"`
	Shop           string `json:"shop"`
	CheckoutID     string `json:"checkout_id"`
	CheckoutToken  string `json:"checkout_token"`
	CreatedAt      int64  `json:"created_at"`
	BlobCachedJson []byte `json:"blob_cached_json"`
}

type ShopifyOrderPaid struct {
	ID             int64  `json:"id"`
	CartToken      string `json:"cart_token"`
	CheckoutID     string `json:"checkout_id"`
	CheckoutToken  string `json:"checkout_token"`
	Shop           string `json:"shop"`
	BlobCachedJson []byte `json:"blob_cached_json"`
	CreatedAt      int64  `json:"created_at"`
}

var sqsService *sqs.SQS
var sqsQueueURL string

func init() {
	// Initialize the SQS service client and session once
	sqsService, sqsQueueURL, _ = ConnectViewSqs()
}

func TestingRoutes(route *gin.Engine) {
	testing := route.Group("/testing")
	testing.GET("/fill-view-queue", FillViewQueueHandler)
	testing.GET("/fill-webhook-queue", FillWebhookQueueHandler)
}

func ConnectViewSqs() (*sqs.SQS, string, error) {
	QueueUrl := "https://sqs.us-east-2.amazonaws.com/671717256085/because-test-queue"
	Region := "us-east-2"
	println("QueueUrl", QueueUrl)
	println("Region", Region)

	sess, err := session.NewSession(&aws.Config{
		Region:     aws.String(Region),
		MaxRetries: aws.Int(5),
	})
	if err != nil {
		log.Warning("Failed to connect upto SQS ", err)
		sentry.CaptureMessage("Failed to connect upto SQS :" + err.Error())
		return nil, "", err
	}

	svc := sqs.New(sess)

	return svc, QueueUrl, nil
}

func ScheduleCampaignViewProcessing(cv *CampaignView) {

	message := fmt.Sprintf(`{"cmd": "campaign_view", "org_id": %d, "shop_name": "%s", "created_at": %d, "site_visitor_id": "%s", "cart_token": "%s", "campaign_id": %d, "product_id": %d, "playbook_id": %d}`,
		cv.OrgId, cv.PartialShopName, cv.CreatedAt, cv.SiteVisitorId, cv.CartToken, cv.CampaignId, cv.ProductId, cv.PlaybookId)

	// Send message
	send_params := &sqs.SendMessageInput{
		MessageBody: aws.String(message),
		QueueUrl:    aws.String(sqsQueueURL),
	}
	_, err1 := sqsService.SendMessage(send_params)
	if err1 != nil {
		log.Warning("SQS Sent message for campaign", fmt.Sprint(cv.CampaignId), "failed :", err1)
		return
	}
}

func ScheduleShopifyCartCreateProcessing(cart *ShopifyCartCreate) {
	message := fmt.Sprintf(`{"cmd": "shopify_cart_create", "id": %d, "cart_token": "%s", "shop": "%s", "created_at": %d}`,
		cart.Id, cart.CartToken, cart.Shop, cart.CreatedAt)

	// Send message
	send_params := &sqs.SendMessageInput{
		MessageBody: aws.String(message),
		QueueUrl:    aws.String(sqsQueueURL),
	}
	_, err1 := sqsService.SendMessage(send_params)
	if err1 != nil {
		log.Warning("SQS Sent message for ShopifyCartCreate ID", cart.Id, "failed:", err1)
		return
	}
}

func ScheduleShopifyOrderCreateProcessing(order *ShopifyOrderCreate) {
	message := fmt.Sprintf(`{"cmd": "shopify_order_create", "ID": %d, "cart_token": "%s", "shop": "%s", "checkout_id": "%s", "checkout_token": "%s", "created_at": %d}`,
		order.ID, order.CartToken, order.Shop, order.CheckoutID, order.CheckoutToken, order.CreatedAt)

	// Send message
	send_params := &sqs.SendMessageInput{
		MessageBody: aws.String(message),
		QueueUrl:    aws.String(sqsQueueURL),
	}
	_, err1 := sqsService.SendMessage(send_params)
	if err1 != nil {
		log.Warning("SQS Sent message for ShopifyOrderCreate ID", order.ID, "failed:", err1)
		return
	}
}

func ScheduleShopifyOrderPaidProcessing(orderPaid *ShopifyOrderPaid) {
	message := fmt.Sprintf(`{"cmd": "shopify_order_paid", "ID": %d, "cart_token": "%s", "checkout_id": "%s", "checkout_token": "%s", "shop": "%s", "created_at": %d}`,
		orderPaid.ID, orderPaid.CartToken, orderPaid.CheckoutID, orderPaid.CheckoutToken, orderPaid.Shop, orderPaid.CreatedAt)

	// Send message
	send_params := &sqs.SendMessageInput{
		MessageBody: aws.String(message),
		QueueUrl:    aws.String(sqsQueueURL),
	}
	_, err1 := sqsService.SendMessage(send_params)
	if err1 != nil {
		log.Warning("SQS Sent message for ShopifyOrderPaid ID", orderPaid.ID, "failed:", err1)
		return
	}
}

// Function to generate a random string of a specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rand.Seed(time.Now().UnixNano())
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// Function to send a fake CampaignView
func SendFakeView() {
	cv := CampaignView{
		OrgId:           0,                         // Fake org ID
		PartialShopName: randomString(10),          // Generate a random shop name (10 characters)
		SiteVisitorId:   randomString(8),           // Generate a random site visitor ID (8 characters)
		CartToken:       randomString(12),          // Generate a random cart token (12 characters)
		CampaignId:      uint(rand.Intn(1000)),     // Generate a random campaign ID (up to 999)
		ProductId:       uint(rand.Intn(10000)),    // Generate a random product ID (up to 9999)
		PlaybookId:      uint(rand.Intn(100)),      // Generate a random playbook ID (up to 99)
		CreatedAt:       time.Now().Unix() - 86400, // Fake timestamp (one day ago in seconds)
	}

	// Send the fake CampaignView to the queue
	ScheduleCampaignViewProcessing(&cv)
}

func SendFakeShopifyCartCreate() {
	cart := ShopifyCartCreate{
		Id:             0,                         // Fake ID
		CartToken:      randomString(12),          // Generate a random cart token (12 characters)
		Shop:           randomString(10),          // Generate a random shop name (10 characters)
		BlobCachedJson: []byte("fake_json_data"),  // Fake JSON data
		CreatedAt:      time.Now().Unix() - 86400, // Fake timestamp (one day ago in seconds)
	}

	// Send the fake ShopifyCartCreate object to the queue
	ScheduleShopifyCartCreateProcessing(&cart)
}

func SendFakeShopifyOrderCreate() {
	order := ShopifyOrderCreate{
		ID:             0,                         // Fake ID
		CartToken:      randomString(12),          // Generate a random cart token (12 characters)
		Shop:           randomString(10),          // Generate a random shop name (10 characters)
		CheckoutID:     randomString(8),           // Generate a random checkout ID (8 characters)
		CheckoutToken:  randomString(16),          // Generate a random checkout token (16 characters)
		BlobCachedJson: []byte("fake_json_data"),  // Fake JSON data
		CreatedAt:      time.Now().Unix() - 86400, // Fake timestamp (one day ago in seconds)
	}

	// Send the fake ShopifyOrderCreate object to the queue
	ScheduleShopifyOrderCreateProcessing(&order)
}

func SendFakeShopifyOrderPaid() {
	orderPaid := ShopifyOrderPaid{
		ID:             0,                         // Fake ID
		CartToken:      randomString(12),          // Generate a random cart token (12 characters)
		CheckoutID:     randomString(8),           // Generate a random checkout ID (8 characters)
		CheckoutToken:  randomString(16),          // Generate a random checkout token (16 characters)
		Shop:           randomString(10),          // Generate a random shop name (10 characters)
		BlobCachedJson: []byte("fake_json_data"),  // Fake JSON data
		CreatedAt:      time.Now().Unix() - 86400, // Fake timestamp (one day ago in seconds)
	}

	// Send the fake ShopifyOrderPaid object to the queue
	ScheduleShopifyOrderPaidProcessing(&orderPaid)
}

func FillViewQueueHandler(c *gin.Context) {
	fmt.Println("Starting to fill the test view queue...")
	// Call the FillTestViewQueue function to fill the test view queue
	FillTestViewQueue()
	fmt.Println("Filling the test view queue completed.")

	// Respond with a success message or appropriate response
	c.JSON(200, gin.H{"message": "Filling test view queue started"})
}

func FillWebhookQueueHandler(c *gin.Context) {
	fmt.Println("Starting to fill the test webhook queue...")
	cartCreateCount := 100000
	orderCreateCount := 100000
	orderPaidCount := 100000
	FillTestWebhookQueue(cartCreateCount, orderCreateCount, orderPaidCount)
	fmt.Println("Filling the test webhook queue completed.")

	// Respond with a success message or appropriate response
	c.JSON(200, gin.H{"message": "Filling test webhook queue started"})
}

func FillTestViewQueue() {
	// Set this to however many views you want to send
	const viewsPerDay = 100000
	const numGoroutines = 10 // Number of goroutines to use for parallel processing

	// Calculate the number of views each goroutine should handle
	viewsPerRoutine := viewsPerDay / numGoroutines

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Send the specified number of fake views from each goroutine
			fmt.Println("Sending fake views from a goroutine...")
			for j := 0; j < viewsPerRoutine; j++ {
				SendFakeView()
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
	fmt.Println("All goroutines have completed sending fake views.")
}

func FillTestWebhookQueue(cartCount, orderCount, orderPaidCount int) {
	const numGoroutines = 10 // Number of goroutines to use for parallel processing

	// Calculate the number of each type of data each goroutine should handle
	cartPerRoutine := cartCount / numGoroutines
	orderPerRoutine := orderCount / numGoroutines
	orderPaidPerRoutine := orderPaidCount / numGoroutines

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Send the specified number of cart creates from each goroutine
			for j := 0; j < cartPerRoutine; j++ {
				SendFakeShopifyCartCreate()
			}

			// Send the specified number of order creates from each goroutine
			for j := 0; j < orderPerRoutine; j++ {
				SendFakeShopifyOrderCreate()
			}

			// Send the specified number of order paid data from each goroutine
			for j := 0; j < orderPaidPerRoutine; j++ {
				SendFakeShopifyOrderPaid()
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
	fmt.Println("All goroutines have completed sending data to the queue.")
}
