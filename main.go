package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	// Parse command line arguments
	showVersion := flag.Bool("version", false, "Show version information and exit")
	configFile := flag.String("config", "", "Path to JSON configuration file")
	flag.Parse()

	// Show version information if requested
	if *showVersion {
		fmt.Println(GetVersionInfo())
		return
	}

	if *configFile == "" {
		log.Fatal("Please specify config file with -config flag")
	}

	// Read and parse configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize statistics for each probe
	stats := make([]*ProbeStats, len(config.Probes))
	for i, probe := range config.Probes {
		stats[i] = &ProbeStats{
			Probe:      probe,
			RTTRecords: make([]RTTRecord, 0),
		}
	}

	// Create CloudWatch client if configured
	var cwClient *CloudWatchClient
	if config.Conf.CloudWatch.Namespace != "" {
		cwClient, err = NewCloudWatchClient(&config.Conf.CloudWatch)
		if err != nil {
			log.Printf("Failed to create CloudWatch client: %v", err)
		} else {
			log.Printf("CloudWatch client initialized for region/namespace: %s/%s",
				config.Conf.CloudWatch.Region, config.Conf.CloudWatch.Namespace)
		}
	}

	// Create ping manager
	pingManager, err := NewPingManager(stats, config.Probes)
	if err != nil {
		log.Fatalf("Failed to create ping manager: %v", err)
	}
	defer pingManager.Close()

	// Start ping workers for each probe
	ctx := make(chan struct{})
	var wg sync.WaitGroup

	for i, probe := range config.Probes {
		wg.Add(1)
		go func(p Probe, probeID int, stat *ProbeStats) {
			defer wg.Done()
			pingWorker(p, probeID, stat, pingManager, ctx, config.Conf.Probe.Interval)
		}(probe, i, stats[i])
	}

	// Start reporting worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		reportingWorker(cwClient, stats, time.Duration(config.Conf.Reporting.Interval)*time.Millisecond, ctx)

	}()

	// Start cloudwatch worker
	wg.Add(1)
	go func() {
		defer wg.Done()

		if cwClient != nil && cwClient.client != nil {
			cloudWatchWorker(cwClient, time.Duration(config.Conf.CloudWatch.Interval)*time.Millisecond, ctx)
		}
	}()

	// Wait for interrupt or completion
	wg.Wait()
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	return &config, err
}

func pingWorker(probe Probe, probeID int, stats *ProbeStats, pingManager *PingManager, ctx chan struct{}, pingInterval int) {
	ticker := time.NewTicker(time.Duration(pingInterval) * time.Millisecond)
	defer ticker.Stop()

	// Use probeID as part of ICMP ID to avoid conflicts
	id := (os.Getpid() & 0xff) | (probeID << 8)
	seq := 0

	log.Printf("Starting ping worker for %s (%s -> %s) (ID: %d, Interval: %dms)",
		probe.Name, getSourceDisplay(probe.Src), probe.Dst, id, pingInterval)

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			seq++
			rtt, err := pingManager.SendPing(probe, id, seq, time.Duration(probe.Timeout)*time.Millisecond, probeID)
			if seq >= (1<<16 - 1) {
				seq = 0
			}
			if err != nil {
				stats.AddFailure()
				if !strings.Contains(err.Error(), "timeout") {
					log.Printf("Ping failed %s (%s -> %s): %v",
						probe.Name, getSourceDisplay(probe.Src), probe.Dst, err)
				}
			} else {
				stats.AddRTT(rtt)
			}
		}
	}
}

func reportingWorker(cwClient *CloudWatchClient, stats []*ProbeStats, interval time.Duration, ctx chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			var now = time.Now().UTC()
			fmt.Printf("\n=== Ping Statistics Report ===\n")
			fmt.Printf("Timestamp: %s\n", now.Format("2006-01-02 15:04:05 UTC"))
			fmt.Printf("%-20s %-15s %-15s %-10s %-10s %-10s %-8s %-8s\n",
				"Name", "Source", "Target", "Min(ms)", "Max(ms)", "Avg(ms)", "Count", "Failures")
			fmt.Println(strings.Repeat("-", 100))

			for _, stat := range stats {
				if cwClient != nil {
					cwClient.PutMetrics(stat)
				}

				// Calculate statistics and clear records
				min, max, avg, count, failures := stat.CalculateAndClear()

				minMs := 0.0
				maxMs := 0.0
				avgMs := 0.0

				if count > 0 {
					minMs = float64(min.Nanoseconds()) / 1e6
					maxMs = float64(max.Nanoseconds()) / 1e6
					avgMs = float64(avg.Nanoseconds()) / 1e6
				}

				// Get display name, fallback to target if name is empty
				displayName := stat.Probe.Name
				if displayName == "" {
					displayName = stat.Probe.Dst
				}

				fmt.Printf("%-20s %-15s %-15s %-10.2f %-10.2f %-10.2f %-8d %-8d\n",
					displayName,
					getSourceDisplay(stat.Probe.Src),
					stat.Probe.Dst,
					minMs,
					maxMs,
					avgMs,
					count,
					failures)
			}
			fmt.Println()
		}
	}
}

func cloudWatchWorker(cwClient *CloudWatchClient, interval time.Duration, ctx chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			cwClient.PublishMetrics()
		}
	}
}
