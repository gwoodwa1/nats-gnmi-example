# Telemetry Data Streaming with NATS and GNMI

## Overview

This project aims to facilitate real-time telemetry data streaming using NATS as the message broker and GNMI for network device interaction. It is comprised of two main components written in Go: a publisher and a subscriber.

### NATS

[NATS](https://nats.io/) is employed as a lightweight and high-performance messaging system (message broker) that securely exchanges messages (here, telemetry data) between distributed systems (the publisher and subscriber in our context). 

#### Ports
- **NATS Default Port**: `4222`

### Components

1. **Publisher**: The publisher collects telemetry data from network devices using the GNMI protocol and forwards this data to a specific topic on the NATS server.

2. **Subscriber**: The subscriber listens to the NATS server on a specific topic, retrieves the messages (telemetry data), and can be utilized to analyze or store this data.

## Getting Started

![image](https://github.com/gwoodwa1/nats-gnmi-example/assets/63735312/088af2e9-b187-44d2-b59a-e9c88405991a)


### Prerequisites

Ensure that you have the following installed and set up:

- [Go](https://golang.org/doc/install) (at least version 1.16)
- Use the `docker-compose` file in the repo to spin up a container of NATS.io using `docker-compose up`
- The `creds.env` file is for your username and password for the network device. Take appropriate measures to protect your creditentials.

### Configuration

The project allows configurations such as specifying the NATS server URL, telemetry topics, GNMI paths, and more through a `config.yaml` file.

An example configuration is as follows:

```yaml
name: "example-device"
address: "192.168.x.x:xxxx"
insecure: true
skipVerify: true
gzip: true
nats_url: "127.0.0.1:4222"
telemetry_topic: "interface-counters"
gnmi_xpath: "/interfaces/interface[name=*]/state/counters"
encoding: "json_ietf"
listmode: "stream"
subscription_mode: "sample"
sample_interval: 10

# `publisher.go` Documentation

## Overview

The `publisher.go` file is designed to manage the collection and publishing of telemetry data using the GNMI (gNMI) protocol and NATS messaging system. This Go file defines the functionality of the publisher component within our telemetry data streaming project. The core activities handled by the code include establishing connections, subscribing to telemetry data, and publishing this data via NATS.

## Dependencies

- **GNMI**: Leveraged for network device interaction.
- **NATS**: A messaging system that ensures the secure exchange of telemetry data.
- **godotenv**: Utilized to manage environment variable loading.
- **yaml.v3**: A YAML parser and outputter for Go.

## Data Structures

### `Config`

The `Config` structure is used to unmarshal the YAML configuration file and store various configuration parameters for establishing gNMI subscriptions and NATS connections.

### `TelemetryTarget`

`TelemetryTarget` contains the configuration and target information for the GNMI subscription. It also contains authentication credentials for the GNMI server.

## Functions

### `NewTelemetryTarget(ctx context.Context, conf Config, username, password string) (*TelemetryTarget, error)`

This function initializes a new `TelemetryTarget` with the provided context, configuration, username, and password, and sets up a new GNMI target with these parameters.

### `collectTelemetry(ctx context.Context, tt *TelemetryTarget) error`

The `collectTelemetry` function is responsible for initiating the GNMI subscription and handling received subscription responses. It utilizes `tt.Target.Subscribe` to initiate the GNMI subscription and then listens to the response and error channels to process incoming telemetry data or handle potential issues during the subscription. Data received is then published to the NATS server.

### `readConfig(filename string) (Config, error)`

The `readConfig` function is designed to read and unmarshal the YAML configuration file into a `Config` struct. It takes a filename as input and returns a configuration structure and an error (if any).

### `sendToNats(ctx context.Context, natsURL, telemetryData, subject string) error`

The `sendToNats` function handles sending the telemetry data to the NATS server. It establishes a connection with the NATS server and then publishes the data to a specified subject/topic.

## Main Execution Logic

### `func main()`

The `main` function orchestrates the overall execution of the publisher:

1. **Loading Credentials**: Utilizing `godotenv` to load GNMI server credentials from an environment file.
   
2. **Loading Configuration**: Calls `readConfig` to parse the YAML configuration file into a `Config` struct.

3. **Context Management**: Establishes a root context with cancellation functionalities to manage graceful shutdowns on receiving termination signals.

4. **Telemetry Target Initialization**: Invokes `NewTelemetryTarget` to create a new telemetry target using the loaded configuration and credentials.

5. **Telemetry Collection**: Calls `collectTelemetry` to begin the process of collecting telemetry data and publishing it to the NATS server.

---

## Usage

To implement this file as a publisher:

1. Ensure that the GNMI server and NATS server are up and running.
   
2. Verify that the configuration file and environment file containing credentials are properly set up and located in the correct path.

3. Execute the `publisher.go` file:
   ```bash
   go run publisher.go

# `subscriber` Documentation

## Overview

The `subscriber` Go file is designed to manage the subscription to a NATS subject, which in this context represents a channel to receive telemetry data published by the publisher component. The subscriber connects to a NATS server, subscribes to a specified subject, listens for messages, and logs received messages to standard output.

## Dependencies

- **NATS**: Utilized to establish a connection, subscribe to subjects, and handle messaging within the NATS messaging system.

## Main Execution Logic

### `func main()`

The `main` function handles the overall execution logic of the subscriber, including connection, subscription, message handling, and graceful shutdown processes:

1. **Connecting to NATS**: Utilizes `nats.Connect` to establish a connection to the NATS server. `nats.DefaultURL` specifies the URL to the NATS server, which defaults to `"nats://localhost:4222"`.

2. **Subscription to a Subject**: `nc.Subscribe` is used to subscribe to the subject `"interface-counters"`. Upon receiving a message, it triggers the provided callback function, which logs the subject and message content.

3. **Signal Handling**: Initializes a channel and listens for termination signals (SIGINT and SIGTERM) to manage graceful shutdowns.

4. **Message Handling**: The callback function provided in the subscription logs the subject and message content to standard output.

5. **Graceful Shutdown**: Upon receiving a termination signal, the subscriber unsubscribes from the subject and drains the connection, ensuring that any pending messages are processed before exiting.

---

## Usage

To implement the `subscriber`:

1. Ensure that the NATS server is running and accessible.

2. Execute the `subscriber` file:
   ```bash
   go run subscriber.go
