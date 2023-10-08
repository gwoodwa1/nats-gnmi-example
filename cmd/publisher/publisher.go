package main

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	api "github.com/openconfig/gnmic/api"
	"github.com/openconfig/gnmic/formatters"
	target "github.com/openconfig/gnmic/target"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	Name             string `yaml:"name"`
	Address          string `yaml:"address"`
	Insecure         bool   `yaml:"insecure"`
	SkipVerify       bool   `yaml:"skipVerify"`
	Gzip             bool   `yaml:"gzip"`
	NatsURL          string `yaml:"nats_url"`
	Topic            string `yaml:"telemetry_topic"`
	XPath            string `yaml:"gnmi_xpath"`
	Encoding         string `yaml:"encoding"`
	ListMode         string `yaml:"listmode"`
	SubscriptionMode string `yaml:"subscription_mode"`
	SampleInterval   int    `yaml:"sample_interval"`
}

type TelemetryTarget struct {
	Config   Config
	Username string
	Password string
	Target   *target.Target
}

func NewTelemetryTarget(ctx context.Context, conf Config, username, password string) (*TelemetryTarget, error) {
	tt := &TelemetryTarget{
		Config:   conf,
		Username: username,
		Password: password,
	}

	var err error
	tt.Target, err = api.NewTarget(
		api.Name(tt.Config.Name),
		api.Address(tt.Config.Address),
		api.Username(username),
		api.Password(password),
		api.Insecure(tt.Config.Insecure),
		api.SkipVerify(tt.Config.SkipVerify),
		api.Gzip(tt.Config.Gzip),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating target: %w", err)
	}

	// Optionally handle context cancellation logic here.
	go func() {
		<-ctx.Done()
		// Cleanup or close logic for tt.Target, if needed.
	}()

	return tt, nil
}

func collectTelemetry(ctx context.Context, tt *TelemetryTarget) error {
	// Check if the TelemetryTarget and its internal Target are non-nil and initialized.
	if tt == nil || tt.Target == nil {
		return fmt.Errorf("telemetry target or its internal target is not properly initialized")
	}

	// Ensure that a GNMI client is created before subscribing.
	if err := tt.Target.CreateGNMIClient(ctx); err != nil {
		return fmt.Errorf("error creating GNMI client: %w", err)
	}

	// Creating a subscription request.
	subReq, err := api.NewSubscribeRequest(
		api.Encoding(tt.Config.Encoding),
		api.SubscriptionListMode(tt.Config.ListMode),
		api.Subscription(
			api.Path(tt.Config.XPath),
			api.SubscriptionMode(tt.Config.SubscriptionMode),
			api.SampleInterval(time.Duration(tt.Config.SampleInterval)*time.Second),
		))
	if err != nil {
		return fmt.Errorf("error creating subscribe request: %w", err)
	}

	// Handling system signals and context cancellation for graceful shutdown.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigs:
		case <-ctx.Done():
		}
		tt.Target.StopSubscription("sub1")
	}()

	// Start subscription in a goroutine.
	go tt.Target.Subscribe(ctx, subReq, "sub1")

	// Read subscriptions and handle responses or errors.
	subRspChan, subErrChan := tt.Target.ReadSubscriptions()
	for {
		select {
		case rsp := <-subRspChan:
			// Processing subscription response...
			options := &formatters.MarshalOptions{Multiline: true, Indent: " "}
			jsonOutput, err := options.Marshal(rsp.Response, nil)
			if err != nil {
				log.Printf("error with JSON serialization %v", err)
				continue
			}

			if len(jsonOutput) > 0 {
				log.Printf("Event at: %s for %s\n", time.Now().Format("2006-01-02 15:04:05"), tt.Config.Name)
				log.Printf("Debug: JSON Output = %s\n", string(jsonOutput))
				publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err = sendToNats(publishCtx, tt.Config.NatsURL, string(jsonOutput), tt.Config.Topic)
				cancel() // Ensure to cancel the context after use to release resources.
				if err != nil {
					log.Printf("Error sending to NATS: %v", err)
				}
			}
		case <-ctx.Done():
			// Context cancelled, exit function.
			return nil
		case tgErr := <-subErrChan:
			// Log errors from the subscription and decide on further action (continue or return).
			log.Printf("subscription %q stopped: %v", tgErr.SubscriptionName, tgErr.Err)
			continue
		}
	}
}

func readConfig(filename string) (Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("error opening YAML file: %v", err)
	}
	defer file.Close()

	yamlFile, err := io.ReadAll(file)
	if err != nil {
		return Config{}, fmt.Errorf("error reading YAML file: %v", err)
	}

	var conf Config
	err = yaml.Unmarshal(yamlFile, &conf)
	if err != nil {
		return Config{}, fmt.Errorf("Error parsing YAML file: %v", err)
	}

	return conf, nil
}

func sendToNats(ctx context.Context, natsURL, telemetryData, subject string) error {
	// Establish a connection with a timeout.
	nc, err := nats.Connect(
		natsURL,
		nats.Timeout(5*time.Second), // Set a connection timeout.
	)
	if err != nil {
		return fmt.Errorf("error connecting to NATS: %v", err)
	}
	defer nc.Close()

	// Check if context is done before trying to publish to prevent hanging when NATS server is not responsive.
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before sending message: %v", ctx.Err())
	default:
		// Try to send the message.
		if err := nc.Publish(subject, []byte(telemetryData)); err != nil {
			return fmt.Errorf("failed to send message to NATS: %v", err)
		}
		log.Printf("Message sent to NATS on subject: %s", subject)
	}

	return nil
}

func main() {
	// Load credentials from the environment file.
	if err := godotenv.Load("./config/creds.env"); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	username := os.Getenv("GNMI_USER")
	password := os.Getenv("PASSWORD")

	// Load configuration.
	conf, err := readConfig("./config/config.yaml")
	if err != nil {
		log.Fatalf("Could not read config: %v", err)
	}

	// Establish a root context with cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure resources are cleaned up when main exits.

	// Setup channel and notify for SIGINT and SIGTERM signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Launch a goroutine to handle termination signals.
	go func() {
		<-sigs
		log.Println("Received termination signal, shutting down...")
		cancel() // Upon receiving a signal, cancel the root context.
	}()
	// Use NewTelemetryTarget to create the telemetry target
	tt, err := NewTelemetryTarget(ctx, conf, username, password)
	if err != nil {
		log.Fatalf("Failed to create telemetry target: %v", err)
	}
	// Start telemetry collection with the root context.
	collectTelemetry(ctx, tt)
}
