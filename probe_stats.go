package main

import (
	"sync"
	"time"
)

// RTT record with timestamp
type RTTRecord struct {
	RTT       time.Duration
	Timestamp time.Time
}

// Statistics tracking with time-based records
type ProbeStats struct {
	Probe      Probe
	RTTRecords []RTTRecord
	Failures   int64
	mutex      sync.RWMutex
}

func (ps *ProbeStats) AddRTT(rtt time.Duration) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.RTTRecords = append(ps.RTTRecords, RTTRecord{
		RTT:       rtt,
		Timestamp: time.Now().UTC(),
	})
}

func (ps *ProbeStats) AddFailure() {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	ps.Failures++
}

func (ps *ProbeStats) CalculateAndClear() (min, max, avg time.Duration, count, failures int64) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	count = int64(len(ps.RTTRecords))
	failures = ps.Failures

	if count == 0 {
		// Clear failures and return zeros
		ps.Failures = 0
		return 0, 0, 0, 0, failures
	}

	// Calculate statistics
	var total time.Duration
	min = ps.RTTRecords[0].RTT
	max = ps.RTTRecords[0].RTT

	for _, record := range ps.RTTRecords {
		total += record.RTT
		if record.RTT < min {
			min = record.RTT
		}
		if record.RTT > max {
			max = record.RTT
		}
	}

	avg = total / time.Duration(count)

	// Clear records after calculation
	ps.RTTRecords = ps.RTTRecords[:0]
	ps.Failures = 0

	return min, max, avg, count, failures
}

func (ps *ProbeStats) GetCurrentCount() int64 {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	return int64(len(ps.RTTRecords))
}

func (ps *ProbeStats) GetCurrentFailures() int64 {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	return ps.Failures
}
