package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudWatch client
type CloudWatchClient struct {
	client     *cloudwatch.Client
	namespace  string
	metricData []types.MetricDatum
	mutex      sync.RWMutex
}

func NewCloudWatchClient(cwConfig *CloudWatchConfig) (*CloudWatchClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(cwConfig.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	return &CloudWatchClient{
		client:     cloudwatch.NewFromConfig(cfg),
		namespace:  cwConfig.Namespace,
		metricData: []types.MetricDatum{},
	}, nil
}

func (cw *CloudWatchClient) PutMetrics(stat *ProbeStats) error {
	defer cw.mutex.Unlock()
	cw.mutex.Lock()

	if !cw.IsValid() {
		return nil // CloudWatch disabled
	}
	count := len(stat.RTTRecords)
	if count == 0 {
		return nil
	}

	// Common dimensions
	dimensions := []types.Dimension{
		{
			Name:  aws.String("ProbeName"),
			Value: aws.String(stat.Probe.Name),
		},
		{
			Name:  aws.String("Source"),
			Value: aws.String(getSourceDisplay(stat.Probe.Src)),
		},
		{
			Name:  aws.String("Target"),
			Value: aws.String(stat.Probe.Dst),
		},
	}

	for _, rttRecord := range stat.RTTRecords {
		cw.metricData = append(cw.metricData, types.MetricDatum{
			MetricName: aws.String("latency"),
			Value:      aws.Float64(float64(rttRecord.RTT.Nanoseconds()) / 1e6),
			Unit:       types.StandardUnitMilliseconds,
			Timestamp:  &rttRecord.Timestamp,
			Dimensions: dimensions,
		})
	}
	return nil
}

func (cw *CloudWatchClient) PublishMetrics() error {
	defer cw.mutex.Unlock()
	cw.mutex.Lock()

	if !cw.IsValid() {
		return nil // CloudWatch disabled
	}
	count := len(cw.metricData)
	if count == 0 {
		return nil
	}

	// Send metrics in batches (CloudWatch limit is 150 datapoints per request)
	batchSize := 150
	for i := 0; i < len(cw.metricData); i += batchSize {
		end := i + batchSize
		if end > len(cw.metricData) {
			end = len(cw.metricData)
		}

		batch := cw.metricData[i:end]
		_, err := cw.client.PutMetricData(context.TODO(), &cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(cw.namespace),
			MetricData: batch,
		})

		if err != nil {
			log.Printf("Failed to send CloudWatch metrics: %v", err)
			return err
		}

		log.Printf("Sent %d data points to CloudWatch namespace: %s", len(cw.metricData), cw.namespace)
		cw.metricData = []types.MetricDatum{}
	}

	return nil
}

func (cw *CloudWatchClient) IsValid() bool {
	return cw.client != nil && cw.namespace != ""
}
